package indexer_test

import (
	"os"
	"path/filepath"
	"testing"

	"frl-painel-documentos/internal/indexer"
	"frl-painel-documentos/internal/store"
)

// fontePath resolve o caminho da fixture sintética em testdata/, nunca dado
// real de cliente (ver SEGURANCA.md).
func fontePath(t *testing.T) string {
	t.Helper()
	caminho, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fonte_exemplo"))
	if err != nil {
		t.Fatalf("resolver caminho da fixture: %v", err)
	}
	return caminho
}

func abrirBancoTeste(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "teste.db"))
	if err != nil {
		t.Fatalf("abrir banco de teste: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestRunIndexaFixtureCorretamente(t *testing.T) {
	st := abrirBancoTeste(t)

	resumo, err := indexer.Run(fontePath(t), st)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 2 empresas na fixture: "Empresa Exemplo Um LTDA" e "Empresa Exemplo Dois ME".
	// "@CONTROLE TESTE" não conta como empresa.
	if resumo.EmpresasIndexadas != 2 {
		t.Errorf("esperava 2 empresas indexadas, veio %d", resumo.EmpresasIndexadas)
	}
	if resumo.PastasRaizIgnoradas != 1 {
		t.Errorf("esperava 1 pasta raiz ignorada (@CONTROLE TESTE), veio %d", resumo.PastasRaizIgnoradas)
	}

	// Documentos válidos: CNPJ.pdf, Contrato.pdf, Certidao Negativa.pdf,
	// PDF Corrompido.pdf (metadado é válido mesmo que o PDF em si seja
	// corrompido — isso só afeta a extração de texto, não a indexação de
	// metadados) = 4. Ignorados: dados.dbk, certificado.pfx = 2. nota.txt
	// não conta (está dentro de uma pasta raiz ignorada, o walk nem entra lá).
	if resumo.DocumentosIndexados != 4 {
		t.Errorf("esperava 4 documentos indexados, veio %d", resumo.DocumentosIndexados)
	}
	if resumo.DocumentosIgnorados != 2 {
		t.Errorf("esperava 2 documentos ignorados (.dbk e .pfx), veio %d", resumo.DocumentosIgnorados)
	}

	empresas, err := st.ListarEmpresas()
	if err != nil {
		t.Fatalf("listar empresas: %v", err)
	}
	nomes := map[string]bool{}
	for _, e := range empresas {
		nomes[e.Nome] = true
	}
	if !nomes["Empresa Exemplo Um LTDA"] || !nomes["Empresa Exemplo Dois ME"] {
		t.Errorf("empresas esperadas não encontradas, veio %v", nomes)
	}
	if nomes["@CONTROLE TESTE"] {
		t.Errorf("pasta com prefixo @ não deveria virar empresa")
	}

	// Confere que nenhum documento com extensão de certificado ou lixo
	// legado foi catalogado, procurando em todas as empresas.
	for _, e := range empresas {
		docs, err := st.ListarDocumentosPorEmpresa(e.ID)
		if err != nil {
			t.Fatalf("listar documentos de %q: %v", e.Nome, err)
		}
		for _, d := range docs {
			if d.Extensao == ".pfx" || d.Extensao == ".p12" {
				t.Errorf("certificado digital não deveria ter sido catalogado: %q", d.CaminhoRelativo)
			}
			if d.Extensao == ".dbk" || d.Extensao == ".db" || d.Extensao == ".dec" || d.Extensao == ".rec" || d.Extensao == ".frm" {
				t.Errorf("extensão de sistema legado não deveria ter sido catalogada: %q", d.CaminhoRelativo)
			}
		}
	}
}

func TestRunEIdempotenteEDetectaRemocao(t *testing.T) {
	// Copia a fixture para uma pasta temporária, porque este teste apaga um
	// arquivo — nunca mexer em testdata/ (fixture compartilhada por outros
	// testes) nem, é claro, na fonte real.
	fonteTemp := t.TempDir()
	copiarArvore(t, fontePath(t), fonteTemp)

	st := abrirBancoTeste(t)

	resumo1, err := indexer.Run(fonteTemp, st)
	if err != nil {
		t.Fatalf("primeira execução: %v", err)
	}
	if resumo1.DocumentosIndexados != 4 {
		t.Fatalf("esperava 4 documentos na primeira execução, veio %d", resumo1.DocumentosIndexados)
	}

	ativos, err := st.ContarDocumentosAtivos()
	if err != nil {
		t.Fatalf("contar documentos ativos: %v", err)
	}
	if ativos != 4 {
		t.Fatalf("esperava 4 documentos ativos após primeira execução, veio %d", ativos)
	}

	// Remove um arquivo da cópia (nunca da fixture original) e reindexa.
	if err := os.Remove(filepath.Join(fonteTemp, "Empresa Exemplo Um LTDA", "CNPJ.pdf")); err != nil {
		t.Fatalf("remover arquivo da cópia: %v", err)
	}

	resumo2, err := indexer.Run(fonteTemp, st)
	if err != nil {
		t.Fatalf("segunda execução: %v", err)
	}
	if resumo2.DocumentosIndexados != 3 {
		t.Fatalf("esperava 3 documentos vistos na segunda execução, veio %d", resumo2.DocumentosIndexados)
	}
	if resumo2.DocumentosRemovidos != 1 {
		t.Fatalf("esperava 1 documento marcado como removido, veio %d", resumo2.DocumentosRemovidos)
	}

	ativos, err = st.ContarDocumentosAtivos()
	if err != nil {
		t.Fatalf("contar documentos ativos após remoção: %v", err)
	}
	if ativos != 3 {
		t.Fatalf("esperava 3 documentos ativos após remoção, veio %d", ativos)
	}
}

func copiarArvore(t *testing.T, origem, destino string) {
	t.Helper()
	err := filepath.WalkDir(origem, func(caminho string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relativo, err := filepath.Rel(origem, caminho)
		if err != nil {
			return err
		}
		destinoCaminho := filepath.Join(destino, relativo)

		if d.IsDir() {
			return os.MkdirAll(destinoCaminho, 0o755)
		}

		conteudo, err := os.ReadFile(caminho)
		if err != nil {
			return err
		}
		return os.WriteFile(destinoCaminho, conteudo, 0o644)
	})
	if err != nil {
		t.Fatalf("copiar árvore de teste: %v", err)
	}
}
