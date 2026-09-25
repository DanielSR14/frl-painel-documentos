# Painel de Documentos FRL

Painel web interno (Go) para indexar, buscar e visualizar o acervo de documentos de clientes do escritório FRL (contratos sociais, CNPJ, certidões, declarações, certificados digitais) hoje espalhado numa pasta de backup sem busca nem controle de acesso.

Veja o plano completo do projeto em [`PLANO_DE_PROJETO.md`](PLANO_DE_PROJETO.md) — contexto real levantado, escopo por fases, stack técnica e arquitetura. Para comandos do dia a dia, convenções e estado atual, veja [`CLAUDE.md`](CLAUDE.md). **Antes de tocar em qualquer coisa que leia a fonte de dados real, leia [`SEGURANCA.md`](SEGURANCA.md)** — o acervo inclui certificados digitais com chave privada.

## Estrutura (ver `CLAUDE.md` para o estado real de cada pasta)

```
cmd/painel/       ponto de entrada do binário (existe)
internal/indexer/ varredura da fonte + extração de metadados (existe — MVP0)
internal/store/    acesso a SQLite (existe — MVP0)
internal/search/   extração de texto de PDF + índice FTS5 (a partir do MVP1)
internal/web/      handlers HTTP, templates (a partir do MVP1)
internal/auth/     login e log de auditoria (a partir do MVP2)
web/               templates HTML e estáticos (a partir do MVP1)
testdata/          fixtures sintéticas para teste (nunca dado real)
```

**Fora de escopo:** certificados digitais (`.pfx`/`.p12`) e as pastas de controle interno com prefixo `@` (`@DCTFWEB`, `@IRPF`, etc.) — ver `SEGURANCA.md`.

## Rodando localmente

```powershell
go build ./...
go test ./...

# uma vez só: copiar o exemplo e editar com o caminho real da sua máquina
copy .env.example .env

go run ./cmd/painel -indexar-somente
```

Requer Go instalado (ver `go.mod` para a versão mínima).
