// Package web serve o painel: lista de empresas, busca e visualização de
// documento, atrás de login (MVP2). Mesmo com login, este servidor só deve
// fazer bind em endereço de rede local, nunca "0.0.0.0" (ver SEGURANCA.md)
// — não foi hardenizado contra exposição direta à internet (sem HTTPS, sem
// rate limiting de login).
package web

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"frl-painel-documentos/internal/auth"
	"frl-painel-documentos/internal/store"
)

//go:embed templates/*.html
var arquivosTemplate embed.FS

var templates = template.Must(template.ParseFS(arquivosTemplate, "templates/*.html"))

type Servidor struct {
	st    *store.Store
	fonte string
	mux   *http.ServeMux
}

func NovoServidor(st *store.Store, fonte string) *Servidor {
	s := &Servidor{st: st, fonte: fonte}

	protegido := http.NewServeMux()
	protegido.HandleFunc("GET /{$}", s.handleEmpresas)
	protegido.HandleFunc("GET /empresas/{id}", s.handleEmpresa)
	protegido.HandleFunc("GET /documentos/{id}/arquivo", s.handleArquivo)
	protegido.HandleFunc("GET /busca", s.handleBusca)
	protegido.HandleFunc("POST /logout", s.handleLogout)

	raiz := http.NewServeMux()
	raiz.HandleFunc("GET /login", s.handleLoginForm)
	raiz.HandleFunc("POST /login", s.handleLoginSubmit)
	raiz.Handle("/", auth.ExigirLogin(st, protegido))

	s.mux = raiz
	return s
}

func (s *Servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Servidor) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	erro := r.URL.Query().Get("erro") == "1"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "login.html", map[string]any{"Erro": erro}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Servidor) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?erro=1", http.StatusSeeOther)
		return
	}

	token, err := auth.Autenticar(s.st, r.FormValue("nome_usuario"), r.FormValue("senha"))
	if err != nil {
		http.Redirect(w, r, "/login?erro=1", http.StatusSeeOther)
		return
	}
	auth.DefinirCookieSessao(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Servidor) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.NomeCookie); err == nil {
		_ = s.st.ApagarSessao(cookie.Value) // best-effort — mesmo se falhar, o cookie é limpo abaixo
	}
	auth.LimparCookieSessao(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Servidor) handleEmpresas(w http.ResponseWriter, r *http.Request) {
	empresas, err := s.st.ListarEmpresas()
	if err != nil {
		http.Error(w, "erro ao listar empresas", http.StatusInternalServerError)
		return
	}
	renderizar(w, r, "empresas.html", map[string]any{"Empresas": empresas}, "")
}

type grupoDocumentos struct {
	Tipo       string
	Documentos []store.Documento
}

func (s *Servidor) handleEmpresa(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	empresa, err := s.st.ObterEmpresa(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	documentos, err := s.st.ListarDocumentosPorEmpresa(id)
	if err != nil {
		http.Error(w, "erro ao listar documentos", http.StatusInternalServerError)
		return
	}

	renderizar(w, r, "empresa.html", map[string]any{
		"Empresa":          empresa,
		"GruposDocumentos": agruparPorTipo(documentos),
	}, "")
}

func agruparPorTipo(documentos []store.Documento) []grupoDocumentos {
	porTipo := map[string][]store.Documento{}
	for _, d := range documentos {
		porTipo[d.TipoDocumento] = append(porTipo[d.TipoDocumento], d)
	}

	tipos := make([]string, 0, len(porTipo))
	for t := range porTipo {
		tipos = append(tipos, t)
	}
	sort.Strings(tipos)

	grupos := make([]grupoDocumentos, 0, len(tipos))
	for _, t := range tipos {
		grupos = append(grupos, grupoDocumentos{Tipo: t, Documentos: porTipo[t]})
	}
	return grupos
}

func (s *Servidor) handleArquivo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	doc, err := s.st.ObterDocumento(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	caminhoAbsoluto := filepath.Join(s.fonte, doc.CaminhoRelativo)

	// Defesa em profundidade: CaminhoRelativo é sempre calculado por
	// filepath.Rel durante a indexação (nunca vem de entrada do usuário),
	// mas confirmamos aqui que o caminho final continua dentro da fonte
	// antes de servir qualquer arquivo.
	relativoConfirmado, err := filepath.Rel(s.fonte, caminhoAbsoluto)
	if err != nil || strings.HasPrefix(relativoConfirmado, "..") {
		http.Error(w, "caminho inválido", http.StatusForbidden)
		return
	}

	// Log de auditoria ANTES de servir o arquivo, nunca depois (ver
	// SEGURANCA.md) — se não der pra registrar o acesso, não servimos o
	// documento. A rota já está atrás de ExigirLogin, então o usuário
	// sempre está presente no contexto aqui.
	usuario, _ := auth.UsuarioDoContexto(r)
	if err := s.st.RegistrarAcesso(usuario.ID, doc.ID); err != nil {
		http.Error(w, "erro ao registrar acesso", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, caminhoAbsoluto)
}

type resultadoBuscaView struct {
	Documento store.Documento
	Empresa   store.Empresa
	Trecho    string
}

func (s *Servidor) handleBusca(w http.ResponseWriter, r *http.Request) {
	termo := strings.TrimSpace(r.URL.Query().Get("q"))
	if termo == "" {
		renderizar(w, r, "busca.html", map[string]any{
			"Empresas":   []store.Empresa{},
			"Resultados": []resultadoBuscaView{},
		}, "")
		return
	}

	empresas, err := s.st.BuscarEmpresasPorNome(termo)
	if err != nil {
		http.Error(w, "erro ao buscar empresas", http.StatusInternalServerError)
		return
	}

	// Busca por frase literal (não expõe a sintaxe de query do FTS5 ao
	// usuário final) — aspas duplas dentro do termo são escapadas conforme
	// a sintaxe de string do FTS5 (dobradas).
	consultaFTS := `"` + strings.ReplaceAll(termo, `"`, `""`) + `"`
	brutos, err := s.st.Buscar(consultaFTS, 50)
	if err != nil {
		http.Error(w, "erro ao buscar documentos", http.StatusInternalServerError)
		return
	}

	resultados := make([]resultadoBuscaView, 0, len(brutos))
	for _, res := range brutos {
		doc, err := s.st.ObterDocumento(res.DocumentoID)
		if err != nil {
			continue
		}
		empresa, err := s.st.ObterEmpresa(doc.EmpresaID)
		if err != nil {
			continue
		}
		resultados = append(resultados, resultadoBuscaView{Documento: doc, Empresa: empresa, Trecho: res.Trecho})
	}

	renderizar(w, r, "busca.html", map[string]any{
		"TermoBusca": termo,
		"Empresas":   empresas,
		"Resultados": resultados,
	}, termo)
}

func renderizar(w http.ResponseWriter, r *http.Request, nomeConteudo string, dados map[string]any, termoBusca string) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, nomeConteudo, dados); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	usuario, _ := auth.UsuarioDoContexto(r)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := templates.ExecuteTemplate(w, "layout.html", map[string]any{
		"Conteudo":    template.HTML(buf.String()), //nolint:gosec // buf vem só dos nossos templates, já auto-escapados na primeira renderização
		"TermoBusca":  termoBusca,
		"NomeUsuario": usuario.NomeUsuario,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
