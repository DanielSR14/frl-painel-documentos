package search_test

import (
	"path/filepath"
	"strings"
	"testing"

	"frl-painel-documentos/internal/search"
)

func TestExtrairTextoDeUmPdfValido(t *testing.T) {
	caminho := filepath.Join("..", "..", "testdata", "fonte_exemplo",
		"Empresa Exemplo Um LTDA", "Contrato Social", "Contrato.pdf")

	texto, err := search.ExtrairTexto(caminho)
	if err != nil {
		t.Fatalf("ExtrairTexto: %v", err)
	}
	if !strings.Contains(texto, "CONTRATO SOCIAL DE TESTE") {
		t.Errorf("texto extraído não contém o esperado, veio: %q", texto)
	}
}

func TestExtrairTextoDeArquivoNaoEhPdfNaoQuebraOProcessamento(t *testing.T) {
	// CNPJ.pdf na fixture é de propósito um arquivo de texto simples com
	// extensão .pdf (simula um PDF corrompido/malformado real) — a extração
	// deve falhar de forma controlada (erro), nunca travar o processo.
	caminho := filepath.Join("..", "..", "testdata", "fonte_exemplo",
		"Empresa Exemplo Um LTDA", "CNPJ.pdf")

	_, err := search.ExtrairTexto(caminho)
	if err == nil {
		t.Fatalf("esperava erro ao tentar extrair texto de um arquivo que não é PDF válido")
	}
}
