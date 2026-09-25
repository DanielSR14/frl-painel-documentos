package store

import (
	"database/sql"
	"fmt"
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

// RemoverUsuario apaga um usuário e suas sessões ativas. Recusa a remoção
// se o usuário tiver qualquer registro em log_acesso — apagar a linha
// quebraria a rastreabilidade do histórico de auditoria (ver SEGURANCA.md,
// "log de auditoria é dado sensível, mesma regra de acesso restrito").
// Serve pra corrigir um usuário criado por engano, não pra desligar alguém
// que já usou o painel de verdade (isso pediria uma função de desativar,
// que ainda não existe — não construir adiantado sem um caso de uso real).
func (s *Store) RemoverUsuario(nomeUsuario string) error {
	u, err := s.ObterUsuarioPorNome(nomeUsuario)
	if err != nil {
		return err
	}

	var totalAcessos int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM log_acesso WHERE usuario_id = ?`, u.ID).Scan(&totalAcessos); err != nil {
		return err
	}
	if totalAcessos > 0 {
		return fmt.Errorf("usuário %q tem %d registro(s) no log de auditoria — remover apagaria rastro de acesso a documento; troque a senha em vez de remover", nomeUsuario, totalAcessos)
	}

	if _, err := s.DB.Exec(`DELETE FROM sessoes WHERE usuario_id = ?`, u.ID); err != nil {
		return err
	}
	_, err = s.DB.Exec(`DELETE FROM usuarios WHERE id = ?`, u.ID)
	return err
}
