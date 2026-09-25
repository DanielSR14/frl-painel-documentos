package store

// ResultadoBusca é um documento encontrado pela busca full-text, com um
// trecho (snippet) do texto ao redor do termo buscado.
type ResultadoBusca struct {
	DocumentoID int64
	Trecho      string
}

// IndexarTextoDocumento grava (ou substitui) o texto extraído de um
// documento no índice de busca. Chamar de novo para o mesmo documentoID
// substitui a entrada anterior — idempotente, seguro de rodar em toda
// reindexação.
func (s *Store) IndexarTextoDocumento(documentoID int64, texto string) error {
	if _, err := s.DB.Exec(`DELETE FROM documentos_busca WHERE documento_id = ?`, documentoID); err != nil {
		return err
	}
	if texto == "" {
		return nil // nada a indexar (PDF sem camada de texto) — não é erro
	}
	_, err := s.DB.Exec(
		`INSERT INTO documentos_busca (texto, documento_id) VALUES (?, ?)`,
		texto, documentoID,
	)
	return err
}

// DocumentosPdfPendentesDeBusca lista documentos .pdf ativos que ainda não
// têm entrada em documentos_busca — usado para processar a extração de
// texto incrementalmente entre execuções (não reprocessar tudo sempre).
func (s *Store) DocumentosPdfPendentesDeBusca() ([]Documento, error) {
	rows, err := s.DB.Query(`
		SELECT d.id, d.empresa_id, d.nome_arquivo, d.caminho_relativo, d.extensao,
		       d.tipo_documento, d.tamanho_bytes, d.modificado_em, d.indexado_em, d.removido_em
		FROM documentos d
		WHERE d.extensao = '.pdf'
		  AND d.removido_em IS NULL
		  AND d.id NOT IN (SELECT documento_id FROM documentos_busca)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var documentos []Documento
	for rows.Next() {
		var d Documento
		if err := rows.Scan(
			&d.ID, &d.EmpresaID, &d.NomeArquivo, &d.CaminhoRelativo, &d.Extensao,
			&d.TipoDocumento, &d.TamanhoBytes, &d.ModificadoEm, &d.IndexadoEm, &d.RemovidoEm,
		); err != nil {
			return nil, err
		}
		documentos = append(documentos, d)
	}
	return documentos, rows.Err()
}

// Buscar procura `consulta` (sintaxe de query do FTS5) no texto indexado e
// devolve os documentos ativos correspondentes, com um trecho de contexto.
func (s *Store) Buscar(consulta string, limite int) ([]ResultadoBusca, error) {
	rows, err := s.DB.Query(`
		SELECT b.documento_id, snippet(documentos_busca, 0, '[', ']', '...', 12)
		FROM documentos_busca b
		JOIN documentos d ON d.id = b.documento_id
		WHERE documentos_busca MATCH ? AND d.removido_em IS NULL
		ORDER BY rank
		LIMIT ?
	`, consulta, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resultados []ResultadoBusca
	for rows.Next() {
		var r ResultadoBusca
		if err := rows.Scan(&r.DocumentoID, &r.Trecho); err != nil {
			return nil, err
		}
		resultados = append(resultados, r)
	}
	return resultados, rows.Err()
}

// BuscarEmpresasPorNome procura empresas cujo nome contém `termo`
// (case-insensitive) — busca simples, não usa FTS5 (é sobre poucas
// centenas de linhas, LIKE é suficiente e mais simples).
func (s *Store) BuscarEmpresasPorNome(termo string) ([]Empresa, error) {
	rows, err := s.DB.Query(
		`SELECT id, nome, pasta_relativa FROM empresas WHERE nome LIKE '%' || ? || '%' ORDER BY nome`,
		termo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var empresas []Empresa
	for rows.Next() {
		var e Empresa
		if err := rows.Scan(&e.ID, &e.Nome, &e.PastaRelativa); err != nil {
			return nil, err
		}
		empresas = append(empresas, e)
	}
	return empresas, rows.Err()
}

// ObterDocumento busca um único documento ativo por id — usado pelo handler
// que serve o arquivo (internal/web) para resolver o caminho relativo.
func (s *Store) ObterDocumento(id int64) (Documento, error) {
	var d Documento
	row := s.DB.QueryRow(`
		SELECT id, empresa_id, nome_arquivo, caminho_relativo, extensao,
		       tipo_documento, tamanho_bytes, modificado_em, indexado_em, removido_em
		FROM documentos WHERE id = ? AND removido_em IS NULL
	`, id)
	err := row.Scan(
		&d.ID, &d.EmpresaID, &d.NomeArquivo, &d.CaminhoRelativo, &d.Extensao,
		&d.TipoDocumento, &d.TamanhoBytes, &d.ModificadoEm, &d.IndexadoEm, &d.RemovidoEm,
	)
	return d, err
}

// ObterEmpresa busca uma única empresa por id.
func (s *Store) ObterEmpresa(id int64) (Empresa, error) {
	var e Empresa
	row := s.DB.QueryRow(`SELECT id, nome, pasta_relativa FROM empresas WHERE id = ?`, id)
	err := row.Scan(&e.ID, &e.Nome, &e.PastaRelativa)
	return e, err
}
