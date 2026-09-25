// Package auth cuida de login (usuário/senha com hash bcrypt), sessão
// (token opaco em cookie, validado contra o store) e do bootstrap do
// primeiro usuário. Log de auditoria de acesso a documento fica no
// próprio store (RegistrarAcesso), chamado pelo internal/web.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"frl-painel-documentos/internal/store"
)

// DuracaoSessao é fixa, sem renovação automática — decisão deliberada de
// simplicidade pro MVP2: expira 12h após o login, força novo login no dia
// seguinte. Ver PLANO_DE_PROJETO.md seção 9 se isso precisar mudar.
const DuracaoSessao = 12 * time.Hour

var ErrCredenciaisInvalidas = errors.New("usuário ou senha inválidos")

func HashSenha(senha string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.DefaultCost)
	return string(b), err
}

func senhaConfere(hash, senha string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(senha)) == nil
}

// Autenticar confere usuário/senha e, se corretos, cria uma sessão nova,
// devolvendo o token a ser colocado no cookie. Não distingue "usuário não
// existe" de "senha errada" no erro devolvido — nunca vazar qual dos dois
// foi o motivo pra quem está tentando logar.
func Autenticar(st *store.Store, nomeUsuario, senha string) (token string, err error) {
	usuario, err := st.ObterUsuarioPorNome(nomeUsuario)
	if err != nil {
		return "", ErrCredenciaisInvalidas
	}
	if !senhaConfere(usuario.HashSenha, senha) {
		return "", ErrCredenciaisInvalidas
	}

	token, err = gerarToken()
	if err != nil {
		return "", err
	}
	if err := st.CriarSessao(token, usuario.ID, time.Now().Add(DuracaoSessao)); err != nil {
		return "", err
	}
	return token, nil
}

func gerarToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GarantirUsuarioInicial cria um usuário se, e só se, ainda não existir
// nenhum usuário cadastrado — bootstrap de conveniência via .env
// (PAINEL_USUARIO_INICIAL/PAINEL_SENHA_INICIAL). Nunca sobrescreve um
// usuário já existente. nomeUsuario/senha vazios são um no-op silencioso
// (permite deixar as variáveis em branco no .env.example).
func GarantirUsuarioInicial(st *store.Store, nomeUsuario, senha string) error {
	if nomeUsuario == "" || senha == "" {
		return nil
	}
	n, err := st.ContarUsuarios()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := HashSenha(senha)
	if err != nil {
		return err
	}
	_, err = st.CriarUsuario(nomeUsuario, hash)
	return err
}

// CriarUsuario cria um usuário novo com a senha já em texto puro (hasheada
// aqui). Usado pelo comando `-criar-usuario` do cmd/painel.
func CriarUsuario(st *store.Store, nomeUsuario, senha string) error {
	hash, err := HashSenha(senha)
	if err != nil {
		return err
	}
	_, err = st.CriarUsuario(nomeUsuario, hash)
	return err
}

// AlterarSenha troca a senha de um usuário já existente.
func AlterarSenha(st *store.Store, nomeUsuario, novaSenha string) error {
	hash, err := HashSenha(novaSenha)
	if err != nil {
		return err
	}
	return st.AtualizarSenhaUsuario(nomeUsuario, hash)
}
