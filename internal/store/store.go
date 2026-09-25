// Package store é a única camada que fala SQL neste projeto — nenhum outro
// pacote deve executar SQL diretamente (ver CLAUDE.md, seção Arquitetura).
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var arquivosMigracao embed.FS

type Store struct {
	DB *sql.DB
}

func Open(caminho string) (*Store, error) {
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		return nil, fmt.Errorf("abrir banco %q: %w", caminho, err)
	}
	// SQLite não lida bem com escrita concorrente de múltiplas conexões;
	// como o indexer roda sozinho (não é o servidor web do MVP1), uma
	// conexão é suficiente e evita "database is locked".
	db.SetMaxOpenConns(1)

	st := &Store{DB: db}
	if err := st.migrar(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrar banco: %w", err)
	}
	return st, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrar() error {
	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		versao INTEGER PRIMARY KEY,
		aplicada_em DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	entradas, err := arquivosMigracao.ReadDir("migrations")
	if err != nil {
		return err
	}
	nomes := make([]string, 0, len(entradas))
	for _, e := range entradas {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			nomes = append(nomes, e.Name())
		}
	}
	sort.Strings(nomes)

	for _, nome := range nomes {
		versao, err := versaoDoNomeArquivo(nome)
		if err != nil {
			return fmt.Errorf("migration %q: %w", nome, err)
		}

		var jaAplicada bool
		row := s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE versao = ?)`, versao)
		if err := row.Scan(&jaAplicada); err != nil {
			return err
		}
		if jaAplicada {
			continue
		}

		sqlBytes, err := arquivosMigracao.ReadFile(path.Join("migrations", nome))
		if err != nil {
			return err
		}

		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("aplicar %q: %w", nome, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (versao) VALUES (?)`, versao); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// versaoDoNomeArquivo extrai o prefixo numérico de "0001_init.sql" -> 1.
func versaoDoNomeArquivo(nome string) (int, error) {
	prefixo, _, ok := strings.Cut(nome, "_")
	if !ok {
		return 0, fmt.Errorf("nome de migration sem prefixo numérico: %s", nome)
	}
	return strconv.Atoi(prefixo)
}
