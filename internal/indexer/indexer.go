// Package indexer varre a pasta de origem dos documentos e popula o store
// com metadados. Nunca escreve, move, renomeia ou apaga nada dentro da
// fonte — só leitura de nome/tamanho/data (ver CLAUDE.md, "Fonte de dados").
package indexer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"frl-painel-documentos/internal/store"
)

// prefixoIgnorado marca pastas de controle interno do escritório (ex:
// "@CERTIFICADOS DIGITAIS", "@CONTROLE CND") — não são empresa cliente e
// ficam fora do escopo deste indexer.
const prefixoIgnorado = "@"

type Resumo struct {
	EmpresasIndexadas   int
	PastasRaizIgnoradas int
	DocumentosIndexados int
	DocumentosIgnorados int
	DocumentosRemovidos int64
}

// Run varre recursivamente `fonte`, tratando cada pasta de primeiro nível
// (exceto as com prefixo "@") como uma Empresa, e cada arquivo dentro dela
// como um Documento em `st`.
func Run(fonte string, st *store.Store) (Resumo, error) {
	var resumo Resumo
	inicioExecucao := time.Now().UTC()

	entradasRaiz, err := os.ReadDir(fonte)
	if err != nil {
		return resumo, fmt.Errorf("ler pasta de origem: %w", err)
	}

	for _, entradaEmpresa := range entradasRaiz {
		if !entradaEmpresa.IsDir() {
			continue // arquivo solto na raiz da fonte, fora do escopo do MVP0
		}

		nomeEmpresa := entradaEmpresa.Name()
		if strings.HasPrefix(nomeEmpresa, prefixoIgnorado) {
			resumo.PastasRaizIgnoradas++
			continue
		}

		empresaID, err := st.UpsertEmpresa(nomeEmpresa, nomeEmpresa)
		if err != nil {
			return resumo, fmt.Errorf("indexar empresa %q: %w", nomeEmpresa, err)
		}
		resumo.EmpresasIndexadas++

		caminhoEmpresa := filepath.Join(fonte, nomeEmpresa)
		err = filepath.WalkDir(caminhoEmpresa, func(caminho string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(d.Name()))
			if extensaoIgnorada(ext) {
				resumo.DocumentosIgnorados++
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			caminhoRelativo, err := filepath.Rel(fonte, caminho)
			if err != nil {
				return err
			}

			nomePastaPai := filepath.Base(filepath.Dir(caminho))
			tipo := inferirTipoDocumento(nomePastaPai, d.Name())

			doc := store.Documento{
				EmpresaID:       empresaID,
				NomeArquivo:     d.Name(),
				CaminhoRelativo: caminhoRelativo,
				Extensao:        ext,
				TipoDocumento:   tipo,
				TamanhoBytes:    info.Size(),
				ModificadoEm:    info.ModTime(),
				IndexadoEm:      inicioExecucao,
			}
			if err := st.UpsertDocumento(doc); err != nil {
				return fmt.Errorf("indexar documento %q: %w", caminhoRelativo, err)
			}
			resumo.DocumentosIndexados++
			return nil
		})
		if err != nil {
			return resumo, err
		}
	}

	removidos, err := st.MarcarAusentesComoRemovidos(inicioExecucao)
	if err != nil {
		return resumo, fmt.Errorf("marcar documentos removidos: %w", err)
	}
	resumo.DocumentosRemovidos = removidos

	return resumo, nil
}
