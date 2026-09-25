// Comando painel roda a indexação do acervo de documentos e, por padrão,
// sobe o painel web read-only (MVP1). Ainda sem autenticação — só deve
// rodar em rede local (ver SEGURANCA.md).
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"frl-painel-documentos/internal/indexer"
	"frl-painel-documentos/internal/search"
	"frl-painel-documentos/internal/store"
	"frl-painel-documentos/internal/web"
)

func main() {
	indexarSomente := flag.Bool("indexar-somente", false, "roda a indexação e a extração de texto, e sai sem subir o servidor web")
	pularBusca := flag.Bool("pular-busca", false, "pula a extração de texto de PDF (só reindexa metadados) — útil pra iterar rápido em desenvolvimento")
	flag.Parse()

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("aviso: não foi possível carregar .env: %v", err)
	}

	fonte := os.Getenv("PAINEL_FONTE_DOCUMENTOS")
	if fonte == "" {
		log.Fatal("PAINEL_FONTE_DOCUMENTOS não definido — copie .env.example para .env e ajuste o caminho para a pasta real de documentos")
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
