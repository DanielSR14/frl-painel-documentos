package search_test

import (
	"path/filepath"
	"testing"

	"frl-painel-documentos/internal/indexer"
	"frl-painel-documentos/internal/search"
	"frl-painel-documentos/internal/store"
)

// TestIndexarPendentesNaoQuebraComPdfCorrompido é uma regressão: a
// biblioteca de extração de texto (github.com/ledongthuc/pdf) demonstrou dar
// panic — não devolver um erro normal — ao processar um PDF real malformado
// (descoberto rodando contra dados reais em 2026-09-25, ver CLAUDE.md
// "Armadilhas conhecidas"). "PDF Corrompido.pdf" na fixture reproduz o
// mesmo tipo de inconsistência de referência interna. Este teste garante
// que IndexarPendentes sobrevive a isso e contabiliza como erro, em vez de
// derrubar o processamento em lote inteiro.
func TestIndexarPendentesNaoQuebraComPdfCorrompido(t *testing.T) {
	fonte, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fonte_exemplo"))
	if err != nil {
		t.Fatalf("resolver fonte: %v", err)
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "teste.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	if _, err := indexer.Run(fonte, st); err != nil {
		t.Fatalf("indexer.Run: %v", err)
	}

	resumo, err := search.IndexarPendentes(fonte, st, 4, nil)
	if err != nil {
		t.Fatalf("IndexarPendentes não deveria retornar erro (nem panicar): %v", err)
	}

	// O ponto deste teste é só "não crashar" — o resultado exato do PDF
	// corrompido (erro, texto vazio, ou até algum texto parcial recuperado
	// via leitura best-effort) não é determinístico o suficiente pra
	// travar num número específico; o que importa é que os 4 documentos
	// foram processados e a soma bate, sem panic escapando da função.
	if resumo.Processados != 4 {
		t.Errorf("esperava 4 documentos processados, veio %d", resumo.Processados)
	}
	if soma := resumo.ComTexto + resumo.SemTexto + resumo.Erros; soma != resumo.Processados {
		t.Errorf("ComTexto(%d) + SemTexto(%d) + Erros(%d) = %d, deveria ser igual a Processados(%d)",
			resumo.ComTexto, resumo.SemTexto, resumo.Erros, soma, resumo.Processados)
	}
	if resumo.ComTexto < 1 {
		t.Errorf("esperava pelo menos 1 documento com texto extraído (Contrato.pdf), veio %d", resumo.ComTexto)
	}
}
