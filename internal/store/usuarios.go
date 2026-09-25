package store

import (
	"database/sql"
	"time"
)

type Usuario struct {
	ID          int64
	NomeUsuario string
	HashSenha   string
	CriadoEm    time.Time
}

func (s *Store) CriarUsuario(nomeUsuario, hashSenha string) (int64, error) {
	res, err := s.DB.Exec(
		`INSERT INTO usuarios (nome_usuario, hash_senha) VALUES (?, ?)`,
		nomeUsuario, hashSenha,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ObterUsuarioPorNome(nomeUsuario string) (Usuario, error) {
	var u Usuario
	row := s.DB.QueryRow(
		`SELECT id, nome_usuario, hash_senha, criado_em FROM usuarios WHERE nome_usuario = ?`,
		nomeUsuario,
	)
	err := row.Scan(&u.ID, &u.NomeUsuario, &u.HashSenha, &u.CriadoEm)
	return u, err
}

// AtualizarSenhaUsuario troca o hash de senha de um usuário já existente.
// Devolve sql.ErrNoRows se o usuário não existir.
func (s *Store) AtualizarSenhaUsuario(nomeUsuario, hashSenha string) error {
	res, err := s.DB.Exec(
		`UPDATE usuarios SET hash_senha = ? WHERE nome_usuario = ?`,
		hashSenha, nomeUsuario,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ContarUsuarios() (int64, error) {
	var n int64
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM usuarios`).Scan(&n)
	return n, err
}
