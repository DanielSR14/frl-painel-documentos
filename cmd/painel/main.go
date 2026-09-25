// Comando painel roda a indexação do acervo de documentos. O modo servidor
// web ainda não existe — chega no MVP1 (ver PLANO_DE_PROJETO.md).
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"frl-painel-documentos/internal/indexer"
	"frl-painel-documentos/internal/store"
)

func main() {
	indexarSomente := flag.Bool("indexar-somente", false, "roda só a indexação e sai (único modo disponível no MVP0)")
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

	if !*indexarSomente {
		fmt.Println("\nModo servidor web ainda não existe (chega no MVP1) — rodando só a indexação.")
	}
}
