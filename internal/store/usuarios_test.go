package store_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"frl-painel-documentos/internal/store"
)

func TestCriarUsuarioEObterPorNome(t *testing.T) {
	st := abrirBancoTeste(t)

	id, err := st.CriarUsuario("joao", "hash-fake")
	if err != nil {
		t.Fatalf("criar usuario: %v", err)
	}

	u, err := st.ObterUsuarioPorNome("joao")
	if err != nil {
		t.Fatalf("obter usuario: %v", err)
	}
	if u.ID != id || u.HashSenha != "hash-fake" {
		t.Errorf("usuario obtido não bate: %+v", u)
	}
}

func TestObterUsuarioInexistente(t *testing.T) {
	st := abrirBancoTeste(t)

	_, err := st.ObterUsuarioPorNome("ninguem")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("esperava sql.ErrNoRows, veio %v", err)
	}
}

func TestAtualizarSenhaUsuario(t *testing.T) {
	st := abrirBancoTeste(t)

	if _, err := st.CriarUsuario("joao", "hash-antigo"); err != nil {
		t.Fatalf("criar usuario: %v", err)
	}
	if err := st.AtualizarSenhaUsuario("joao", "hash-novo"); err != nil {
		t.Fatalf("atualizar senha: %v", err)
	}

	u, err := st.ObterUsuarioPorNome("joao")
	if err != nil {
		t.Fatalf("obter usuario: %v", err)
	}
	if u.HashSenha != "hash-novo" {
		t.Errorf("esperava hash-novo, veio %q", u.HashSenha)
	}
}

func TestAtualizarSenhaUsuarioInexistente(t *testing.T) {
	st := abrirBancoTeste(t)

	err := st.AtualizarSenhaUsuario("ninguem", "hash")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("esperava sql.ErrNoRows, veio %v", err)
	}
}

func TestContarUsuarios(t *testing.T) {
	st := abrirBancoTeste(t)

	n, err := st.ContarUsuarios()
	if err != nil || n != 0 {
		t.Fatalf("esperava 0 usuarios, veio %d (err=%v)", n, err)
	}

	if _, err := st.CriarUsuario("joao", "hash"); err != nil {
		t.Fatalf("criar usuario: %v", err)
	}
	n, err = st.ContarUsuarios()
	if err != nil || n != 1 {
		t.Fatalf("esperava 1 usuario, veio %d (err=%v)", n, err)
	}
}

func TestSessaoValidaEExpirada(t *testing.T) {
	st := abrirBancoTeste(t)

	usuarioID, err := st.CriarUsuario("joao", "hash")
	if err != nil {
		t.Fatalf("criar usuario: %v", err)
	}

	if err := st.CriarSessao("token-valido", usuarioID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("criar sessao valida: %v", err)
	}
	if err := st.CriarSessao("token-expirado", usuarioID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("criar sessao expirada: %v", err)
	}

	sc, err := st.ObterSessaoValida("token-valido")
	if err != nil {
		t.Fatalf("obter sessao valida: %v", err)
	}
	if sc.UsuarioID != usuarioID || sc.NomeUsuario != "joao" {
		t.Errorf("sessao obtida não bate: %+v", sc)
	}

	_, err = st.ObterSessaoValida("token-expirado")
	if err == nil {
		t.Errorf("esperava erro para sessão expirada, não veio nenhum")
	}

	_, err = st.ObterSessaoValida("token-inexistente")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("esperava sql.ErrNoRows pra token inexistente, veio %v", err)
	}
}

func TestApagarSessao(t *testing.T) {
	st := abrirBancoTeste(t)

	usuarioID, err := st.CriarUsuario("joao", "hash")
	if err != nil {
		t.Fatalf("criar usuario: %v", err)
	}
	if err := st.CriarSessao("token", usuarioID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("criar sessao: %v", err)
	}

	if err := st.ApagarSessao("token"); err != nil {
		t.Fatalf("apagar sessao: %v", err)
	}

	_, err = st.ObterSessaoValida("token")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("esperava sql.ErrNoRows após apagar, veio %v", err)
	}
}

func TestRegistrarAcesso(t *testing.T) {
	st := abrirBancoTeste(t)

	usuarioID, err := st.CriarUsuario("joao", "hash")
	if err != nil {
		t.Fatalf("criar usuario: %v", err)
	}
	empresaID, err := st.UpsertEmpresa("Empresa Exemplo LTDA", "Empresa Exemplo LTDA")
	if err != nil {
		t.Fatalf("upsert empresa: %v", err)
	}
	agora := time.Now().UTC()
	if err := st.UpsertDocumento(store.Documento{
		EmpresaID: empresaID, NomeArquivo: "A.pdf", CaminhoRelativo: "Empresa Exemplo LTDA/A.pdf",
		Extensao: ".pdf", TipoDocumento: "outro", TamanhoBytes: 1, ModificadoEm: agora, IndexadoEm: agora,
	}); err != nil {
		t.Fatalf("upsert documento: %v", err)
	}
	documentos, err := st.ListarDocumentosPorEmpresa(empresaID)
	if err != nil || len(documentos) != 1 {
		t.Fatalf("listar documentos: %v (len=%d)", err, len(documentos))
	}

	if err := st.RegistrarAcesso(usuarioID, documentos[0].ID); err != nil {
		t.Fatalf("registrar acesso: %v", err)
	}

	var total int
	row := st.DB.QueryRow(`SELECT COUNT(*) FROM log_acesso WHERE usuario_id = ? AND documento_id = ?`, usuarioID, documentos[0].ID)
	if err := row.Scan(&total); err != nil {
		t.Fatalf("consultar log_acesso: %v", err)
	}
	if total != 1 {
		t.Errorf("esperava 1 registro de acesso, veio %d", total)
	}
}
