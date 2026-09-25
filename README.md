# Painel de Documentos FRL

Painel web interno (Go) para indexar, buscar e visualizar o acervo de documentos de clientes do escritório FRL (contratos sociais, CNPJ, certidões, declarações, certificados digitais) hoje espalhado numa pasta de backup sem busca nem controle de acesso.

Veja o plano completo do projeto em [`PLANO_DE_PROJETO.md`](PLANO_DE_PROJETO.md) — contexto real levantado, escopo por fases, stack técnica e arquitetura. Para comandos do dia a dia, convenções e estado atual, veja [`CLAUDE.md`](CLAUDE.md). **Antes de tocar em qualquer coisa que leia a fonte de dados real, leia [`SEGURANCA.md`](SEGURANCA.md)** — o acervo inclui certificados digitais com chave privada.

## Estrutura (ver `CLAUDE.md` para o estado real de cada pasta)

```
cmd/painel/       ponto de entrada do binário
internal/indexer/ varredura da fonte + extração de metadados
internal/store/    acesso a SQLite (schema, FTS5, usuários, sessões, auditoria)
internal/search/   extração de texto de PDF + orquestração da busca
internal/auth/     login, sessão, middleware de autenticação
internal/web/      handlers HTTP + templates embutidos
testdata/          fixtures sintéticas para teste (nunca dado real)
```

**Fora de escopo:** certificados digitais (`.pfx`/`.p12`) e as pastas de controle interno com prefixo `@` (`@DCTFWEB`, `@IRPF`, etc.) — ver `SEGURANCA.md`.

## Rodando localmente

Jeito mais simples: dar duplo clique em **`START.BAT`** (ou rodar pelo terminal). Ele copia o `.env.example` pra `.env` na primeira vez (e abre pra você editar o caminho da fonte), compila e sobe o painel em `http://127.0.0.1:8080`.

**Antes do primeiro uso, é preciso criar um usuário** (o painel exige login desde o MVP2 — não existe modo sem login):

```powershell
go build -o painel.exe ./cmd/painel
.\painel.exe -criar-usuario "seu.nome"
```

Manualmente (build/test/indexação):

```powershell
go build ./...
go test ./...

# uma vez só: copiar o exemplo e editar com o caminho real da sua máquina
copy .env.example .env

go run ./cmd/painel -indexar-somente
```

Requer Go instalado (ver `go.mod` para a versão mínima).
