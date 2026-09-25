package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"frl-painel-documentos/internal/store"
)

func abrirBancoTeste(t *testing.T) *store.Store {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "teste.db")
	st, err := store.Open(caminho)
	if err != nil {
		t.Fatalf("abrir banco de teste: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestUpsertEmpresaCriaEAtualiza(t *testing.T) {
	st := abrirBancoTeste(t)

	id1, err := st.UpsertEmpresa("Empresa Exemplo LTDA", "Empresa Exemplo LTDA")
	if err != nil {
		t.Fatalf("upsert inicial: %v", err)
	}

	id2, err := st.UpsertEmpresa("Empresa Exemplo LTDA (renomeada)", "Empresa Exemplo LTDA")
	if err != nil {
		t.Fatalf("upsert de atualização: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("esperava mesmo id ao reindexar a mesma pasta, veio %d e %d", id1, id2)
	}

	empresas, err := st.ListarEmpresas()
	if err != nil {
		t.Fatalf("listar empresas: %v", err)
	}
	if len(empresas) != 1 {
		t.Fatalf("esperava 1 empresa, veio %d", len(empresas))
	}
	if empresas[0].Nome != "Empresa Exemplo LTDA (renomeada)" {
		t.Fatalf("nome não foi atualizado, veio %q", empresas[0].Nome)
	}
}

func TestUpsertDocumentoEMarcarAusentesComoRemovidos(t *testing.T) {
	st := abrirBancoTeste(t)

	empresaID, err := st.UpsertEmpresa("Empresa Exemplo LTDA", "Empresa Exemplo LTDA")
	if err != nil {
		t.Fatalf("upsert empresa: %v", err)
	}

	primeiraPassada := time.Now().UTC()
	doc := store.Documento{
		EmpresaID:       empresaID,
		NomeArquivo:     "CNPJ.pdf",
		CaminhoRelativo: "Empresa Exemplo LTDA/CNPJ.pdf",
		Extensao:        ".pdf",
		TipoDocumento:   "cnpj",
		TamanhoBytes:    1234,
		ModificadoEm:    primeiraPassada,
		IndexadoEm:      primeiraPassada,
	}
	if err := st.UpsertDocumento(doc); err != nil {
		t.Fatalf("upsert documento: %v", err)
	}

	n, err := st.ContarDocumentosAtivos()
	if err != nil {
		t.Fatalf("contar documentos: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperava 1 documento ativo, veio %d", n)
	}

	// Segunda passada do indexer: o arquivo sumiu da fonte, então não é
	// re-upsertado — só a marcação de ausência roda.
	segundaPassada := primeiraPassada.Add(time.Second)
	removidos, err := st.MarcarAusentesComoRemovidos(segundaPassada)
	if err != nil {
		t.Fatalf("marcar ausentes: %v", err)
	}
	if removidos != 1 {
		t.Fatalf("esperava 1 documento marcado como removido, veio %d", removidos)
	}

	n, err = st.ContarDocumentosAtivos()
	if err != nil {
		t.Fatalf("contar documentos após remoção: %v", err)
	}
	if n != 0 {
		t.Fatalf("esperava 0 documentos ativos após remoção, veio %d", n)
	}

	// O arquivo reaparece numa terceira passada: upsert deve limpar RemovidoEm.
	terceiraPassada := segundaPassada.Add(time.Second)
	doc.IndexadoEm = terceiraPassada
	if err := st.UpsertDocumento(doc); err != nil {
		t.Fatalf("upsert documento (reaparecido): %v", err)
	}

	n, err = st.ContarDocumentosAtivos()
	if err != nil {
		t.Fatalf("contar documentos após reaparecer: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperava 1 documento ativo após reaparecer, veio %d", n)
	}
}

func TestListarDocumentosPorEmpresaIgnoraRemovidos(t *testing.T) {
	st := abrirBancoTeste(t)

	empresaID, err := st.UpsertEmpresa("Empresa Exemplo LTDA", "Empresa Exemplo LTDA")
	if err != nil {
		t.Fatalf("upsert empresa: %v", err)
	}

	agora := time.Now().UTC()
	for _, nome := range []string{"CNPJ.pdf", "Contrato Social.pdf"} {
		doc := store.Documento{
			EmpresaID:       empresaID,
			NomeArquivo:     nome,
			CaminhoRelativo: "Empresa Exemplo LTDA/" + nome,
			Extensao:        ".pdf",
			TipoDocumento:   "outro",
			TamanhoBytes:    100,
			ModificadoEm:    agora,
			IndexadoEm:      agora,
		}
		if err := st.UpsertDocumento(doc); err != nil {
			t.Fatalf("upsert documento %q: %v", nome, err)
		}
	}

	if _, err := st.MarcarAusentesComoRemovidos(agora.Add(time.Second)); err != nil {
		t.Fatalf("marcar ausentes: %v", err)
	}

	documentos, err := st.ListarDocumentosPorEmpresa(empresaID)
	if err != nil {
		t.Fatalf("listar documentos: %v", err)
	}
	if len(documentos) != 0 {
		t.Fatalf("esperava 0 documentos (todos removidos), veio %d", len(documentos))
	}
}
