package auth

import (
	"context"
	"net/http"

	"frl-painel-documentos/internal/store"
)

const NomeCookie = "painel_sessao"

type InfoUsuario struct {
	ID          int64
	NomeUsuario string
}

type chaveUsuario struct{}

// ExigirLogin protege o handler seguinte: sem cookie de sessão válido,
// redireciona pra /login. Com sessão válida, injeta InfoUsuario no
// contexto da requisição (ver UsuarioDoContexto) e segue normalmente.
func ExigirLogin(st *store.Store, proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(NomeCookie)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		sessao, err := st.ObterSessaoValida(cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), chaveUsuario{}, InfoUsuario{
			ID:          sessao.UsuarioID,
			NomeUsuario: sessao.NomeUsuario,
		})
		proximo.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UsuarioDoContexto lê o usuário autenticado injetado por ExigirLogin.
// O segundo retorno é false fora de uma rota protegida (ex: /login).
func UsuarioDoContexto(r *http.Request) (InfoUsuario, bool) {
	v, ok := r.Context().Value(chaveUsuario{}).(InfoUsuario)
	return v, ok
}

// DefinirCookieSessao grava o cookie de sessão após um login bem-sucedido.
func DefinirCookieSessao(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     NomeCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(DuracaoSessao.Seconds()),
		// Secure fica false de propósito: o painel roda em HTTP simples na
		// rede local (ver SEGURANCA.md) — Secure=true faria o navegador
		// nunca enviar o cookie e ninguém conseguiria logar.
	})
}

// LimparCookieSessao remove o cookie de sessão do navegador (logout). Não
// apaga a sessão no store — isso é responsabilidade de quem chama, via
// store.ApagarSessao(tokenAntigo), pra não deixar sessão órfã válida.
func LimparCookieSessao(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     NomeCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
