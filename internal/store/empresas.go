package store

// UpsertEmpresa cria a empresa se não existir, ou atualiza o nome se a pasta
// já era conhecida — identificada por PastaRelativa (estável entre passadas
// do indexer), não pelo nome.
func (s *Store) UpsertEmpresa(nome, pastaRelativa string) (int64, error) {
	_, err := s.DB.Exec(`
		INSERT INTO empresas (nome, pasta_relativa)
		VALUES (?, ?)
		ON CONFLICT(pasta_relativa) DO UPDATE SET
			nome = excluded.nome,
			atualizado_em = CURRENT_TIMESTAMP
	`, nome, pastaRelativa)
	if err != nil {
		return 0, err
	}

	var id int64
	row := s.DB.QueryRow(`SELECT id FROM empresas WHERE pasta_relativa = ?`, pastaRelativa)
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) ListarEmpresas() ([]Empresa, error) {
	rows, err := s.DB.Query(`SELECT id, nome, pasta_relativa FROM empresas ORDER BY nome`)
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
