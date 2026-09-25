CREATE TABLE empresas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nome TEXT NOT NULL,
    pasta_relativa TEXT NOT NULL UNIQUE,
    criado_em DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    atualizado_em DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE documentos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    empresa_id INTEGER NOT NULL REFERENCES empresas(id),
    nome_arquivo TEXT NOT NULL,
    caminho_relativo TEXT NOT NULL UNIQUE,
    extensao TEXT NOT NULL,
    tipo_documento TEXT NOT NULL DEFAULT 'outro',
    tamanho_bytes INTEGER NOT NULL,
    modificado_em DATETIME NOT NULL,
    indexado_em DATETIME NOT NULL,
    removido_em DATETIME
);

CREATE INDEX idx_documentos_empresa ON documentos(empresa_id);
CREATE INDEX idx_documentos_removido_em ON documentos(removido_em);
