package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"frl-painel-documentos/internal/auth"
	"frl-painel-documentos/internal/indexer"
	"frl-painel-documentos/internal/search"
	"frl-painel-documentos/internal/store"
	"frl-painel-documentos/internal/web"
)

const usuarioTeste = "usuario.teste"
const senhaTeste = "senha-de-teste-123"

// prepararServidor indexa a fixture sintética de testdata/ (nunca dado real
// de cliente) num banco temporário, cria um usuário de teste já autenticado,
// e devolve um Servidor pronto para receber requisições via httptest.
func prepararServidor(t *testing.T) (servidor *web.Servidor, st *store.Store, tokenSessao string) {
	t.Helper()
	fonte, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fonte_exemplo"))
	if err != nil {
		t.Fatalf("resolver fonte: %v", err)
	}

	st, err = store.Open(filepath.Join(t.TempDir(), "teste.db"))
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

	if err := auth.CriarUsuario(st, usuarioTeste, senhaTeste); err != nil {
		t.Fatalf("criar usuario de teste: %v", err)
	}
	token, err := auth.Autenticar(st, usuarioTeste, senhaTeste)
	if err != nil {
		t.Fatalf("autenticar usuario de teste: %v", err)
	}

	return web.NovoServidor(st, fonte), st, token
}

func pedir(servidor *web.Servidor, caminho, tokenSessao string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, caminho, nil)
	if tokenSessao != "" {
		req.AddCookie(&http.Cookie{Name: auth.NomeCookie, Value: tokenSessao})
	}
	rec := httptest.NewRecorder()
	servidor.ServeHTTP(rec, req)
	return rec
}

func TestRotaProtegidaSemLoginRedirecionaParaLogin(t *testing.T) {
	servidor, _, _ := prepararServidor(t)

	rec := pedir(servidor, "/", "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperava redirect (303), veio %d", rec.Code)
	}
	if local := rec.Header().Get("Location"); local != "/login" {
		t.Errorf("esperava redirect pra /login, veio %q", local)
	}
}

func TestLoginComCredenciaisErradasNaoAutentica(t *testing.T) {
	servidor, _, _ := prepararServidor(t)

	form := strings.NewReader("nome_usuario=" + usuarioTeste + "&senha=senha-errada")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	servidor.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login?erro=1" {
		t.Fatalf("esperava redirect pra /login?erro=1, veio status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("login mal sucedido não deveria setar cookie de sessão")
	}
}

func TestLoginComCredenciaisCorretasAutenticaEPermiteAcesso(t *testing.T) {
	servidor, _, _ := prepararServidor(t)

	form := strings.NewReader("nome_usuario=" + usuarioTeste + "&senha=" + senhaTeste)
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	servidor.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("esperava redirect pra /, veio status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.NomeCookie {
		t.Fatalf("esperava cookie de sessão setado, veio %+v", cookies)
	}

	rec2 := pedir(servidor, "/", cookies[0].Value)
	if rec2.Code != http.StatusOK {
		t.Fatalf("esperava acesso liberado com a sessão recém-criada, veio %d", rec2.Code)
	}
}

func TestListaDeEmpresas(t *testing.T) {
	servidor, _, token := prepararServidor(t)

	rec := pedir(servidor, "/", token)
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
	servidor, st, token := prepararServidor(t)

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

	rec := pedir(servidor, fmt.Sprintf("/empresas/%d", idEmpresaUm), token)
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
	servidor, _, token := prepararServidor(t)

	rec := pedir(servidor, "/empresas/999999", token)
	if rec.Code != http.StatusNotFound {
		t.Errorf("esperava 404 para empresa inexistente, veio %d", rec.Code)
	}
}

func TestBuscaPorConteudoDeDocumento(t *testing.T) {
	servidor, _, token := prepararServidor(t)

	rec := pedir(servidor, "/busca?q=CONTRATO+SOCIAL+DE+TESTE", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	corpo := rec.Body.String()
	if !strings.Contains(corpo, "Contrato.pdf") {
		t.Errorf("esperava encontrar Contrato.pdf pelo conteúdo, corpo: %s", corpo)
	}
}

func TestServirArquivoDoDocumentoRegistraAcesso(t *testing.T) {
	servidor, st, token := prepararServidor(t)

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

	rec := pedir(servidor, fmt.Sprintf("/documentos/%d/arquivo", idCnpj), token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Errorf("esperava conteúdo do arquivo no corpo da resposta")
	}

	var totalAcessos int
	row := st.DB.QueryRow(`SELECT COUNT(*) FROM log_acesso WHERE documento_id = ?`, idCnpj)
	if err := row.Scan(&totalAcessos); err != nil {
		t.Fatalf("consultar log_acesso: %v", err)
	}
	if totalAcessos != 1 {
		t.Errorf("esperava 1 registro de acesso no log de auditoria, veio %d", totalAcessos)
	}
}

func TestArquivoSemLoginNaoServeENaoRegistraAcesso(t *testing.T) {
	servidor, st, token := prepararServidor(t)

	empresas, err := st.ListarEmpresas()
	if err != nil {
		t.Fatalf("listar empresas: %v", err)
	}
	documentos, err := st.ListarDocumentosPorEmpresa(empresas[0].ID)
	if err != nil || len(documentos) == 0 {
		t.Fatalf("listar documentos: %v", err)
	}
	_ = token // usado só pra achar um documento válido, não pra esta requisição

	rec := pedir(servidor, fmt.Sprintf("/documentos/%d/arquivo", documentos[0].ID), "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperava redirect pra login sem sessão, veio %d", rec.Code)
	}

	var totalAcessos int
	row := st.DB.QueryRow(`SELECT COUNT(*) FROM log_acesso WHERE documento_id = ?`, documentos[0].ID)
	if err := row.Scan(&totalAcessos); err != nil {
		t.Fatalf("consultar log_acesso: %v", err)
	}
	if totalAcessos != 0 {
		t.Errorf("acesso não autenticado não deveria gerar registro de auditoria, veio %d", totalAcessos)
	}
}

func TestLogoutInvalidaSessao(t *testing.T) {
	servidor, _, token := prepararServidor(t)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.NomeCookie, Value: token})
	rec := httptest.NewRecorder()
	servidor.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("esperava redirect pra /login, veio status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}

	// O mesmo token não deve mais funcionar depois do logout.
	rec2 := pedir(servidor, "/", token)
	if rec2.Code != http.StatusSeeOther {
		t.Errorf("esperava sessão invalidada após logout, mas ainda deu acesso (status %d)", rec2.Code)
	}
}
