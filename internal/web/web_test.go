package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"frl-painel-documentos/internal/indexer"
	"frl-painel-documentos/internal/search"
	"frl-painel-documentos/internal/store"
	"frl-painel-documentos/internal/web"
)

// prepararServidor indexa a fixture sintética de testdata/ (nunca dado real
// de cliente) num banco temporário e devolve um Servidor pronto para
// receber requisições via httptest, junto com o *store.Store (pra buscar
// IDs reais em vez de fixar valores mágicos nos testes).
func prepararServidor(t *testing.T) (*web.Servidor, *store.Store) {
	t.Helper()
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
	if _, err := search.IndexarPendentes(fonte, st, 2, nil); err != nil {
		t.Fatalf("search.IndexarPendentes: %v", err)
	}

	return web.NovoServidor(st, fonte), st
}

func pedir(servidor *web.Servidor, caminho string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, caminho, nil)
	rec := httptest.NewRecorder()
	servidor.ServeHTTP(rec, req)
	return rec
}

func TestListaDeEmpresas(t *testing.T) {
	servidor, _ := prepararServidor(t)

	rec := pedir(servidor, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	corpo := rec.Body.String()
	if !strings.Contains(corpo, "Empresa Exemplo Um LTDA") || !strings.Contains(corpo, "Empresa Exemplo Dois ME") {
		t.Errorf("esperava as duas empresas na lista, corpo: %s", corpo)
	}
	if strings.Contains(corpo, "CONTROLE TESTE") {
		t.Errorf("pasta @ ignorada não deveria aparecer em lugar nenhum: %s", corpo)
	}
}

func TestDetalheDeEmpresaAgrupaDocumentosPorTipo(t *testing.T) {
	servidor, st := prepararServidor(t)

	empresas, err := st.ListarEmpresas()
	if err != nil {
		t.Fatalf("listar empresas: %v", err)
	}
	var idEmpresaUm int64
	for _, e := range empresas {
		if e.Nome == "Empresa Exemplo Um LTDA" {
			idEmpresaUm = e.ID
		}
	}
	if idEmpresaUm == 0 {
		t.Fatalf("empresa de teste não encontrada")
	}

	rec := pedir(servidor, fmt.Sprintf("/empresas/%d", idEmpresaUm))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	corpo := rec.Body.String()
	if !strings.Contains(corpo, "CNPJ.pdf") {
		t.Errorf("esperava CNPJ.pdf listado, corpo: %s", corpo)
	}
	if strings.Contains(corpo, "certificado.pfx") || strings.Contains(corpo, "dados.dbk") {
		t.Errorf("arquivo ignorado pelo indexer não deveria aparecer na página da empresa: %s", corpo)
	}
}

func TestEmpresaInexistenteDevolve404(t *testing.T) {
	servidor, _ := prepararServidor(t)

	rec := pedir(servidor, "/empresas/999999")
	if rec.Code != http.StatusNotFound {
		t.Errorf("esperava 404 para empresa inexistente, veio %d", rec.Code)
	}
}

func TestBuscaPorConteudoDeDocumento(t *testing.T) {
	servidor, _ := prepararServidor(t)

	rec := pedir(servidor, "/busca?q=CONTRATO+SOCIAL+DE+TESTE")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	corpo := rec.Body.String()
	if !strings.Contains(corpo, "Contrato.pdf") {
		t.Errorf("esperava encontrar Contrato.pdf pelo conteúdo, corpo: %s", corpo)
	}
}

func TestServirArquivoDoDocumento(t *testing.T) {
	servidor, st := prepararServidor(t)

	empresas, err := st.ListarEmpresas()
	if err != nil {
		t.Fatalf("listar empresas: %v", err)
	}
	var idEmpresaUm int64
	for _, e := range empresas {
		if e.Nome == "Empresa Exemplo Um LTDA" {
			idEmpresaUm = e.ID
		}
	}
	documentos, err := st.ListarDocumentosPorEmpresa(idEmpresaUm)
	if err != nil {
		t.Fatalf("listar documentos: %v", err)
	}
	var idCnpj int64
	for _, d := range documentos {
		if d.NomeArquivo == "CNPJ.pdf" {
			idCnpj = d.ID
		}
	}
	if idCnpj == 0 {
		t.Fatalf("documento CNPJ.pdf não encontrado")
	}

	rec := pedir(servidor, fmt.Sprintf("/documentos/%d/arquivo", idCnpj))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Errorf("esperava conteúdo do arquivo no corpo da resposta")
	}
}
