// Comando painel indexa o acervo de documentos e sobe o painel web atrás
// de login (MVP2). Só deve rodar em rede local, nunca exposto à internet
// (ver SEGURANCA.md — sem HTTPS, sem rate limiting de tentativa de login).
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/term"

	"frl-painel-documentos/internal/auth"
	"frl-painel-documentos/internal/indexer"
	"frl-painel-documentos/internal/search"
	"frl-painel-documentos/internal/store"
	"frl-painel-documentos/internal/web"
)

func main() {
	indexarSomente := flag.Bool("indexar-somente", false, "roda a indexação e a extração de texto, e sai sem subir o servidor web")
	pularBusca := flag.Bool("pular-busca", false, "pula a extração de texto de PDF (só reindexa metadados) — útil pra iterar rápido em desenvolvimento")
	criarUsuario := flag.String("criar-usuario", "", "cria um usuário novo com o nome dado (pede a senha interativamente) e sai, sem indexar nem subir o servidor")
	alterarSenha := flag.String("alterar-senha", "", "altera a senha de um usuário já existente (pede a senha nova interativamente) e sai")
	flag.Parse()

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("aviso: não foi possível carregar .env: %v", err)
	}

	caminhoBanco := os.Getenv("PAINEL_BANCO_DADOS")
	if caminhoBanco == "" {
		caminhoBanco = "data/painel.db"
	}
	if err := os.MkdirAll(filepath.Dir(caminhoBanco), 0o755); err != nil {
		log.Fatalf("criar pasta do banco: %v", err)
	}

	st, err := store.Open(caminhoBanco)
	if err != nil {
		log.Fatalf("abrir banco: %v", err)
	}
	defer st.Close()

	if *criarUsuario != "" {
		rodarCriarUsuario(st, *criarUsuario)
		return
	}
	if *alterarSenha != "" {
		rodarAlterarSenha(st, *alterarSenha)
		return
	}

	fonte := os.Getenv("PAINEL_FONTE_DOCUMENTOS")
	if fonte == "" {
		log.Fatal("PAINEL_FONTE_DOCUMENTOS não definido — copie .env.example para .env e ajuste o caminho para a pasta real de documentos")
	}

	if err := auth.GarantirUsuarioInicial(st, os.Getenv("PAINEL_USUARIO_INICIAL"), os.Getenv("PAINEL_SENHA_INICIAL")); err != nil {
		log.Fatalf("criar usuário inicial: %v", err)
	}
	if n, err := st.ContarUsuarios(); err != nil {
		log.Fatalf("contar usuários: %v", err)
	} else if n == 0 {
		log.Fatal("nenhum usuário cadastrado ainda — rode: painel.exe -criar-usuario \"seu.nome\"")
	}

	resumo, err := indexer.Run(fonte, st)
	if err != nil {
		log.Fatalf("indexar: %v", err)
	}
	fmt.Printf("Empresas indexadas: %d\n", resumo.EmpresasIndexadas)
	fmt.Printf("Pastas raiz ignoradas (prefixo @): %d\n", resumo.PastasRaizIgnoradas)
	fmt.Printf("Documentos indexados: %d\n", resumo.DocumentosIndexados)
	fmt.Printf("Documentos ignorados (extensão de sistema legado ou certificado digital): %d\n", resumo.DocumentosIgnorados)
	fmt.Printf("Documentos marcados como removidos nesta passada: %d\n", resumo.DocumentosRemovidos)

	if !*pularBusca {
		ultimoRelatado := 0
		aoProgredir := func(processados, total int) {
			// Imprime a cada 500 documentos (ou menos, se o lote for pequeno)
			// pra dar visibilidade em lotes grandes sem inundar o terminal.
			passo := 500
			if processados-ultimoRelatado >= passo || processados == total {
				fmt.Printf("Extraindo texto: %d/%d\n", processados, total)
				ultimoRelatado = processados
			}
		}

		resumoBusca, err := search.IndexarPendentes(fonte, st, 8, aoProgredir)
		if err != nil {
			log.Fatalf("indexar texto para busca: %v", err)
		}
		fmt.Printf("\nExtração de texto — processados: %d, com texto: %d, sem texto (PDF escaneado/sem camada de texto): %d, erros: %d\n",
			resumoBusca.Processados, resumoBusca.ComTexto, resumoBusca.SemTexto, resumoBusca.Erros)
	}

	if *indexarSomente {
		return
	}

	endereco := os.Getenv("PAINEL_ENDERECO")
	if endereco == "" {
		endereco = "127.0.0.1:8080" // nunca 0.0.0.0 por padrão — ver SEGURANCA.md
	}

	servidor := web.NovoServidor(st, fonte)
	fmt.Printf("\nPainel disponível em http://%s\n", endereco)
	if err := http.ListenAndServe(endereco, servidor); err != nil {
		log.Fatalf("servidor web: %v", err)
	}
}

func rodarCriarUsuario(st *store.Store, nomeUsuario string) {
	senha := lerSenhaComConfirmacao()
	if err := auth.CriarUsuario(st, nomeUsuario, senha); err != nil {
		log.Fatalf("criar usuário: %v", err)
	}
	fmt.Printf("Usuário %q criado.\n", nomeUsuario)
}

func rodarAlterarSenha(st *store.Store, nomeUsuario string) {
	if _, err := st.ObterUsuarioPorNome(nomeUsuario); err != nil {
		log.Fatalf("usuário %q não existe", nomeUsuario)
	}
	senha := lerSenhaComConfirmacao()
	if err := auth.AlterarSenha(st, nomeUsuario, senha); err != nil {
		log.Fatalf("alterar senha: %v", err)
	}
	fmt.Printf("Senha de %q atualizada.\n", nomeUsuario)
}

// lerSenhaComConfirmacao pede a senha duas vezes (sem ecoar no terminal,
// via golang.org/x/term) e falha se as duas não baterem.
//
// Importante: o *bufio.Reader do caminho "stdin não é terminal" (abaixo)
// precisa ser criado uma vez só e reaproveitado entre as duas chamadas —
// um bufio.Reader novo por chamada consome do pipe adiantado (lê mais do
// que uma linha de uma vez) e descarta o resto no buffer interno da
// instância anterior, fazendo a segunda leitura dar EOF mesmo com dado
// disponível. Bug real, pego testando com `printf 'a\nb\n' | painel.exe`.
func lerSenhaComConfirmacao() string {
	leitorFallback := bufio.NewReader(os.Stdin)
	senha := lerSenhaOculta("Senha: ", leitorFallback)
	confirmacao := lerSenhaOculta("Confirme a senha: ", leitorFallback)
	if senha != confirmacao {
		log.Fatal("as senhas não coincidem")
	}
	if len(strings.TrimSpace(senha)) < 8 {
		log.Fatal("a senha precisa ter pelo menos 8 caracteres")
	}
	return senha
}

func lerSenhaOculta(prompt string, leitorFallback *bufio.Reader) string {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			log.Fatalf("ler senha: %v", err)
		}
		return string(b)
	}
	// stdin não é um terminal (ex: rodando via script/CI) — cair pra
	// leitura simples de linha, sem esconder a senha.
	linha, err := leitorFallback.ReadString('\n')
	if err != nil {
		log.Fatalf("ler senha: %v", err)
	}
	fmt.Println()
	return strings.TrimRight(linha, "\r\n")
}
