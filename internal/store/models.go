package store

import "time"

type Empresa struct {
	ID            int64
	Nome          string
	PastaRelativa string
}

type Documento struct {
	ID              int64
	EmpresaID       int64
	NomeArquivo     string
	CaminhoRelativo string
	Extensao        string
	TipoDocumento   string
	TamanhoBytes    int64
	ModificadoEm    time.Time
	IndexadoEm      time.Time
	RemovidoEm      *time.Time
}
