package search

import (
	"fmt"
	"path/filepath"
	"sync"

	"frl-painel-documentos/internal/store"
)

type ResumoIndexacao struct {
	Processados int
	ComTexto    int
	SemTexto    int
	Erros       int
}

// IndexarPendentes extrai texto de todo documento PDF ainda sem entrada no
// índice de busca (st.DocumentosPdfPendentesDeBusca) e grava no store.
// A extração roda em paralelo (I/O e CPU-bound); as escritas no SQLite são
// naturalmente serializadas pelo pool de conexão do store (MaxOpenConns=1),
// então não precisa de lock adicional para a parte de persistência.
//
// aoProgredir, se não for nil, é chamado após cada documento processado com
// (processados, total) — útil pra imprimir progresso em lotes grandes (o
// acervo real tem ~20 mil PDFs, alguns escaneados e pesados o suficiente
// pra essa etapa demorar minutos sem nenhum feedback visível). Pode ser
// nil quando o chamador não precisa de progresso (ex: testes). É sempre
// chamado de forma serializada (nunca duas vezes concorrentemente), mesmo
// com concorrencia > 1 — o chamador não precisa de lock próprio.
func IndexarPendentes(fonte string, st *store.Store, concorrencia int, aoProgredir func(processados, total int)) (ResumoIndexacao, error) {
	pendentes, err := st.DocumentosPdfPendentesDeBusca()
	if err != nil {
		return ResumoIndexacao{}, err
	}
	total := len(pendentes)
	if concorrencia < 1 {
		concorrencia = 1
	}

	var (
		mu     sync.Mutex
		resumo ResumoIndexacao
		wg     sync.WaitGroup
	)
	tarefas := make(chan store.Documento)

	for i := 0; i < concorrencia; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for doc := range tarefas {
				texto, err := extrairComRecuperacao(fonte, doc)

				mu.Lock()
				resumo.Processados++
				switch {
				case err != nil:
					resumo.Erros++
				case texto == "":
					resumo.SemTexto++
				default:
					resumo.ComTexto++
				}
				processados := resumo.Processados
				if aoProgredir != nil {
					aoProgredir(processados, total) // chamado sob mu: serializado, sem race no chamador
				}
				mu.Unlock()

				if err != nil {
					continue // arquivo problemático, já contabilizado — segue pro próximo
				}
				if err := st.IndexarTextoDocumento(doc.ID, texto); err != nil {
					mu.Lock()
					resumo.Erros++
					mu.Unlock()
				}
			}
		}()
	}

	for _, doc := range pendentes {
		tarefas <- doc
	}
	close(tarefas)
	wg.Wait()

	return resumo, nil
}

// extrairComRecuperacao isola cada arquivo num recover() próprio: a
// biblioteca de extração de PDF já demonstrou dar panic (não só erro) em
// arquivo real malformado (ver CLAUDE.md, Armadilhas conhecidas). Um PDF
// problemático não pode derrubar o processamento em lote inteiro.
func extrairComRecuperacao(fonte string, doc store.Documento) (texto string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("pânico ao extrair texto de %q: %v", doc.CaminhoRelativo, p)
		}
	}()
	return ExtrairTexto(filepath.Join(fonte, doc.CaminhoRelativo))
}
