// Package web serve o painel read-only do MVP1: lista de empresas, busca e
// visualização de documento. Sem autenticação ainda (chega no MVP2) — por
// isso este servidor só deve fazer bind em endereço de rede local, nunca
// "0.0.0.0" (ver SEGURANCA.md).
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
	s := &Servidor{st: st, fonte: fonte, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /{$}", s.handleEmpresas)
	s.mux.HandleFunc("GET /empresas/{id}", s.handleEmpresa)
	s.mux.HandleFunc("GET /documentos/{id}/arquivo", s.handleArquivo)
	s.mux.HandleFunc("GET /busca", s.handleBusca)
	return s
}

func (s *Servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Servidor) handleEmpresas(w http.ResponseWriter, r *http.Request) {
	empresas, err := s.st.ListarEmpresas()
	if err != nil {
		http.Error(w, "erro ao listar empresas", http.StatusInternalServerError)
		return
	}
	renderizar(w, "empresas.html", map[string]any{"Empresas": empresas}, "")
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

	renderizar(w, "empresa.html", map[string]any{
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
		renderizar(w, "busca.html", map[string]any{
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
	for _, r := range brutos {
		doc, err := s.st.ObterDocumento(r.DocumentoID)
		if err != nil {
			continue
		}
		empresa, err := s.st.ObterEmpresa(doc.EmpresaID)
		if err != nil {
			continue
		}
		resultados = append(resultados, resultadoBuscaView{Documento: doc, Empresa: empresa, Trecho: r.Trecho})
	}

	renderizar(w, "busca.html", map[string]any{
		"TermoBusca": termo,
		"Empresas":   empresas,
		"Resultados": resultados,
	}, termo)
}

func renderizar(w http.ResponseWriter, nomeConteudo string, dados map[string]any, termoBusca string) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, nomeConteudo, dados); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := templates.ExecuteTemplate(w, "layout.html", map[string]any{
		"Conteudo":   template.HTML(buf.String()), //nolint:gosec // buf vem só dos nossos templates, já auto-escapados na primeira renderização
		"TermoBusca": termoBusca,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
