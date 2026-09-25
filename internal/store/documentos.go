package store

import "time"

// UpsertDocumento identifica o documento por CaminhoRelativo (estável entre
// passadas do indexer). Reaparecer depois de ter sido marcado como removido
// limpa RemovidoEm automaticamente.
func (s *Store) UpsertDocumento(d Documento) error {
	_, err := s.DB.Exec(`
		INSERT INTO documentos (
			empresa_id, nome_arquivo, caminho_relativo, extensao,
			tipo_documento, tamanho_bytes, modificado_em, indexado_em, removido_em
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)
		ON CONFLICT(caminho_relativo) DO UPDATE SET
			empresa_id = excluded.empresa_id,
			nome_arquivo = excluded.nome_arquivo,
			extensao = excluded.extensao,
			tipo_documento = excluded.tipo_documento,
			tamanho_bytes = excluded.tamanho_bytes,
			modificado_em = excluded.modificado_em,
			indexado_em = excluded.indexado_em,
			removido_em = NULL
	`,
		d.EmpresaID, d.NomeArquivo, d.CaminhoRelativo, d.Extensao,
		d.TipoDocumento, d.TamanhoBytes, d.ModificadoEm, d.IndexadoEm,
	)
	return err
}

// MarcarAusentesComoRemovidos marca como removidos (soft delete) os
// documentos cujo indexado_em é anterior ao início da execução atual —
// ou seja, que existiam no banco mas não foram vistos nesta passada porque
// sumiram da fonte (a fonte é read-only para este projeto, então "sumir"
// só pode significar que o arquivo foi removido por outra pessoa/processo).
func (s *Store) MarcarAusentesComoRemovidos(inicioExecucao time.Time) (int64, error) {
	res, err := s.DB.Exec(`
		UPDATE documentos
		SET removido_em = CURRENT_TIMESTAMP
		WHERE indexado_em < ? AND removido_em IS NULL
	`, inicioExecucao)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) ContarDocumentosAtivos() (int64, error) {
	var n int64
	row := s.DB.QueryRow(`SELECT COUNT(*) FROM documentos WHERE removido_em IS NULL`)
	err := row.Scan(&n)
	return n, err
}

func (s *Store) ListarDocumentosPorEmpresa(empresaID int64) ([]Documento, error) {
	rows, err := s.DB.Query(`
		SELECT id, empresa_id, nome_arquivo, caminho_relativo, extensao,
		       tipo_documento, tamanho_bytes, modificado_em, indexado_em, removido_em
		FROM documentos
		WHERE empresa_id = ? AND removido_em IS NULL
		ORDER BY nome_arquivo
	`, empresaID)
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
