-- Tabela virtual FTS5 para busca full-text no texto extraído de PDF
-- (internal/search). Não usa "external content" ligada a documentos porque
-- a extração de texto roda numa etapa separada da indexação de metadados —
-- mais simples manter como tabela própria e sincronizar via código
-- (internal/store/busca.go) do que via triggers SQL.
CREATE VIRTUAL TABLE documentos_busca USING fts5(
    texto,
    documento_id UNINDEXED
);
