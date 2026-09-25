package store

import "time"

// CriarSessao normaliza expiraEm para UTC antes de gravar — o driver SQLite
// grava time.Time como texto RFC3339 com o offset local (ex: "-03:00"),
// enquanto CURRENT_TIMESTAMP do SQLite é sempre UTC sem offset. Comparar os
// dois como texto (ObterSessaoValida faz isso) só funciona se ambos os
// lados estiverem em UTC — ver CLAUDE.md, Armadilhas conhecidas.
func (s *Store) CriarSessao(token string, usuarioID int64, expiraEm time.Time) error {
	_, err := s.DB.Exec(
		`INSERT INTO sessoes (token, usuario_id, expira_em) VALUES (?, ?, ?)`,
		token, usuarioID, expiraEm.UTC(),
	)
	return err
}

type SessaoComUsuario struct {
	UsuarioID   int64
	NomeUsuario string
}

// ObterSessaoValida busca uma sessão pelo token, só devolvendo resultado
// (sem erro) se ela ainda não tiver expirado. Sessão expirada ou token
// inexistente resulta em sql.ErrNoRows, do jeito que QueryRow já devolve.
func (s *Store) ObterSessaoValida(token string) (SessaoComUsuario, error) {
	var sc SessaoComUsuario
	row := s.DB.QueryRow(`
		SELECT s.usuario_id, u.nome_usuario
		FROM sessoes s
		JOIN usuarios u ON u.id = s.usuario_id
		WHERE s.token = ? AND s.expira_em > CURRENT_TIMESTAMP
	`, token)
	err := row.Scan(&sc.UsuarioID, &sc.NomeUsuario)
	return sc, err
}

func (s *Store) ApagarSessao(token string) error {
	_, err := s.DB.Exec(`DELETE FROM sessoes WHERE token = ?`, token)
	return err
}
