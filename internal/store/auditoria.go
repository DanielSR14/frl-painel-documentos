package store

// RegistrarAcesso grava que usuarioID acessou documentoID agora. Chamar
// sempre ANTES de servir o conteúdo do documento, nunca depois — se o
// registro falhar, o acesso não deve acontecer (ver SEGURANCA.md).
func (s *Store) RegistrarAcesso(usuarioID, documentoID int64) error {
	_, err := s.DB.Exec(
		`INSERT INTO log_acesso (usuario_id, documento_id) VALUES (?, ?)`,
		usuarioID, documentoID,
	)
	return err
}
