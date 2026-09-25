# Painel de Documentos FRL

Painel web interno (Go) para indexar, buscar e visualizar o acervo de documentos de clientes do escritório FRL (contratos sociais, CNPJ, certidões, declarações, certificados digitais) hoje espalhado numa pasta de backup sem busca nem controle de acesso.

Veja o plano completo do projeto em [`PLANO_DE_PROJETO.md`](PLANO_DE_PROJETO.md) — contexto real levantado, escopo por fases, stack técnica e arquitetura. Para comandos do dia a dia, convenções e estado atual, veja [`CLAUDE.md`](CLAUDE.md). **Antes de tocar em qualquer coisa que leia a fonte de dados real, leia [`SEGURANCA.md`](SEGURANCA.md)** — o acervo inclui certificados digitais com chave privada.

## Estrutura (planejada — ver `CLAUDE.md` para o estado real de cada pasta)

```
cmd/painel/       ponto de entrada do binário
internal/indexer/ varredura da fonte + extração de metadados
internal/store/    acesso a SQLite
internal/search/   extração de texto de PDF + índice FTS5
internal/web/      handlers HTTP, templates
internal/auth/     login e log de auditoria (a partir do MVP2)
web/               templates HTML e estáticos
testdata/          fixtures sintéticas para teste (nunca dado real)
```

## Rodando localmente

*(Ainda não implementado — projeto na Fase 0. Ver "Estado atual" em `CLAUDE.md`.)*

```powershell
go build ./...
go test ./...
$env:PAINEL_FONTE_DOCUMENTOS = "<caminho da pasta de backup>"
go run ./cmd/painel
```

Requer Go instalado (ver `go.mod` para a versão mínima).
