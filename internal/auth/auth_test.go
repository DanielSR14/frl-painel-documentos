package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"frl-painel-documentos/internal/auth"
	"frl-painel-documentos/internal/store"
)

func abrirBancoTeste(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "teste.db"))
	if err != nil {
		t.Fatalf("abrir banco de teste: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestAutenticarComCredenciaisCorretas(t *testing.T) {
	st := abrirBancoTeste(t)
	if err := auth.CriarUsuario(st, "joao", "senha-correta"); err != nil {
		t.Fatalf("criar usuario: %v", err)
	}

	token, err := auth.Autenticar(st, "joao", "senha-correta")
	if err != nil {
		t.Fatalf("autenticar: %v", err)
	}
	if token == "" {
		t.Errorf("esperava um token não vazio")
	}

	sessao, err := st.ObterSessaoValida(token)
	if err != nil {
		t.Fatalf("sessão criada não é válida: %v", err)
	}
	if sessao.NomeUsuario != "joao" {
		t.Errorf("sessão pertence ao usuário errado: %q", sessao.NomeUsuario)
	}
}

func TestAutenticarComSenhaErrada(t *testing.T) {
	st := abrirBancoTeste(t)
	if err := auth.CriarUsuario(st, "joao", "senha-correta"); err != nil {
		t.Fatalf("criar usuario: %v", err)
	}

	_, err := auth.Autenticar(st, "joao", "senha-errada")
	if !errors.Is(err, auth.ErrCredenciaisInvalidas) {
		t.Errorf("esperava ErrCredenciaisInvalidas, veio %v", err)
	}
}

func TestAutenticarComUsuarioInexistente(t *testing.T) {
	st := abrirBancoTeste(t)

	_, err := auth.Autenticar(st, "ninguem", "qualquer-senha")
	if !errors.Is(err, auth.ErrCredenciaisInvalidas) {
		t.Errorf("esperava ErrCredenciaisInvalidas (não deve vazar se o usuário existe), veio %v", err)
	}
}

func TestGarantirUsuarioInicialSoCriaUmaVez(t *testing.T) {
	st := abrirBancoTeste(t)

	if err := auth.GarantirUsuarioInicial(st, "admin", "senha-inicial"); err != nil {
		t.Fatalf("garantir usuario inicial: %v", err)
	}
	n, err := st.ContarUsuarios()
	if err != nil || n != 1 {
		t.Fatalf("esperava 1 usuário, veio %d (err=%v)", n, err)
	}

	// Chamar de novo com credenciais diferentes não deve sobrescrever nada.
	if err := auth.GarantirUsuarioInicial(st, "outro", "outra-senha"); err != nil {
		t.Fatalf("segunda chamada: %v", err)
	}
	n, err = st.ContarUsuarios()
	if err != nil || n != 1 {
		t.Fatalf("esperava continuar com 1 usuário, veio %d (err=%v)", n, err)
	}
	if _, err := st.ObterUsuarioPorNome("admin"); err != nil {
		t.Errorf("usuário original deveria continuar existindo: %v", err)
	}
}

func TestGarantirUsuarioInicialComCredenciaisVaziasNaoFazNada(t *testing.T) {
	st := abrirBancoTeste(t)

	if err := auth.GarantirUsuarioInicial(st, "", ""); err != nil {
		t.Fatalf("não deveria dar erro: %v", err)
	}
	n, err := st.ContarUsuarios()
	if err != nil || n != 0 {
		t.Fatalf("esperava 0 usuários, veio %d (err=%v)", n, err)
	}
}

func TestAlterarSenha(t *testing.T) {
	st := abrirBancoTeste(t)
	if err := auth.CriarUsuario(st, "joao", "senha-antiga"); err != nil {
		t.Fatalf("criar usuario: %v", err)
	}

	if err := auth.AlterarSenha(st, "joao", "senha-nova"); err != nil {
		t.Fatalf("alterar senha: %v", err)
	}

	if _, err := auth.Autenticar(st, "joao", "senha-antiga"); err == nil {
		t.Errorf("senha antiga não deveria mais funcionar")
	}
	if _, err := auth.Autenticar(st, "joao", "senha-nova"); err != nil {
		t.Errorf("senha nova deveria funcionar: %v", err)
	}
}

func TestExigirLoginSemCookieRedirecionaParaLogin(t *testing.T) {
	st := abrirBancoTeste(t)
	protegido := auth.ExigirLogin(st, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler protegido não deveria ser chamado sem sessão válida")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	protegido.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperava redirect (303), veio %d", rec.Code)
	}
	if local := rec.Header().Get("Location"); local != "/login" {
		t.Errorf("esperava redirect pra /login, veio %q", local)
	}
}

func TestExigirLoginComSessaoValidaInjetaUsuarioNoContexto(t *testing.T) {
	st := abrirBancoTeste(t)
	if err := auth.CriarUsuario(st, "joao", "senha"); err != nil {
		t.Fatalf("criar usuario: %v", err)
	}
	token, err := auth.Autenticar(st, "joao", "senha")
	if err != nil {
		t.Fatalf("autenticar: %v", err)
	}

	var usuarioRecebido auth.InfoUsuario
	var okRecebido bool
	protegido := auth.ExigirLogin(st, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usuarioRecebido, okRecebido = auth.UsuarioDoContexto(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.NomeCookie, Value: token})
	rec := httptest.NewRecorder()
	protegido.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	if !okRecebido || usuarioRecebido.NomeUsuario != "joao" {
		t.Errorf("usuário não foi injetado corretamente no contexto: %+v (ok=%v)", usuarioRecebido, okRecebido)
	}
}
