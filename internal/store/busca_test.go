package store_test

import (
	"testing"
	"time"

	"frl-painel-documentos/internal/store"
)

func TestIndexarTextoEBuscar(t *testing.T) {
	st := abrirBancoTeste(t)

	empresaID, err := st.UpsertEmpresa("Empresa Exemplo LTDA", "Empresa Exemplo LTDA")
	if err != nil {
		t.Fatalf("upsert empresa: %v", err)
	}

	agora := time.Now().UTC()
	doc := store.Documento{
		EmpresaID:       empresaID,
		NomeArquivo:     "Contrato.pdf",
		CaminhoRelativo: "Empresa Exemplo LTDA/Contrato.pdf",
		Extensao:        ".pdf",
		TipoDocumento:   "contrato_social",
		TamanhoBytes:    100,
		ModificadoEm:    agora,
		IndexadoEm:      agora,
	}
	if err := st.UpsertDocumento(doc); err != nil {
		t.Fatalf("upsert documento: %v", err)
	}

	documentos, err := st.ListarDocumentosPorEmpresa(empresaID)
	if err != nil || len(documentos) != 1 {
		t.Fatalf("listar documentos: %v (len=%d)", err, len(documentos))
	}
	documentoID := documentos[0].ID

	if err := st.IndexarTextoDocumento(documentoID, "CONTRATO SOCIAL DE TESTE PARA BUSCA FULLTEXT"); err != nil {
		t.Fatalf("indexar texto: %v", err)
	}

	resultados, err := st.Buscar("BUSCA", 10)
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	if len(resultados) != 1 {
		t.Fatalf("esperava 1 resultado, veio %d", len(resultados))
	}
	if resultados[0].DocumentoID != documentoID {
		t.Errorf("documento_id errado no resultado: veio %d, esperava %d", resultados[0].DocumentoID, documentoID)
	}

	// Reindexar substitui, não duplica.
	if err := st.IndexarTextoDocumento(documentoID, "TEXTO ATUALIZADO SEM A PALAVRA ANTIGA"); err != nil {
		t.Fatalf("reindexar texto: %v", err)
	}
	resultados, err = st.Buscar("BUSCA", 10)
	if err != nil {
		t.Fatalf("buscar após reindexar: %v", err)
	}
	if len(resultados) != 0 {
		t.Fatalf("esperava 0 resultados após reindexar com texto diferente, veio %d", len(resultados))
	}
	resultados, err = st.Buscar("ATUALIZADO", 10)
	if err != nil {
		t.Fatalf("buscar texto novo: %v", err)
	}
	if len(resultados) != 1 {
		t.Fatalf("esperava 1 resultado pro texto novo, veio %d", len(resultados))
	}
}

func TestDocumentosPdfPendentesDeBusca(t *testing.T) {
	st := abrirBancoTeste(t)

	empresaID, err := st.UpsertEmpresa("Empresa Exemplo LTDA", "Empresa Exemplo LTDA")
	if err != nil {
		t.Fatalf("upsert empresa: %v", err)
	}

	agora := time.Now().UTC()
	docPdf := store.Documento{
		EmpresaID: empresaID, NomeArquivo: "A.pdf", CaminhoRelativo: "Empresa Exemplo LTDA/A.pdf",
		Extensao: ".pdf", TipoDocumento: "outro", TamanhoBytes: 1, ModificadoEm: agora, IndexadoEm: agora,
	}
	docTxt := store.Documento{
		EmpresaID: empresaID, NomeArquivo: "B.txt", CaminhoRelativo: "Empresa Exemplo LTDA/B.txt",
		Extensao: ".txt", TipoDocumento: "outro", TamanhoBytes: 1, ModificadoEm: agora, IndexadoEm: agora,
	}
	if err := st.UpsertDocumento(docPdf); err != nil {
		t.Fatalf("upsert pdf: %v", err)
	}
	if err := st.UpsertDocumento(docTxt); err != nil {
		t.Fatalf("upsert txt: %v", err)
	}

	pendentes, err := st.DocumentosPdfPendentesDeBusca()
	if err != nil {
		t.Fatalf("pendentes: %v", err)
	}
	if len(pendentes) != 1 || pendentes[0].NomeArquivo != "A.pdf" {
		t.Fatalf("esperava só A.pdf pendente, veio %+v", pendentes)
	}

	if err := st.IndexarTextoDocumento(pendentes[0].ID, "algum texto"); err != nil {
		t.Fatalf("indexar texto: %v", err)
	}

	pendentes, err = st.DocumentosPdfPendentesDeBusca()
	if err != nil {
		t.Fatalf("pendentes após indexar: %v", err)
	}
	if len(pendentes) != 0 {
		t.Fatalf("esperava 0 pendentes após indexar, veio %d", len(pendentes))
	}
}
