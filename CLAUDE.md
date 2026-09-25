# CLAUDE.md — Painel de Documentos FRL

Guia de referência rápida para trabalhar neste repositório. Para o plano de produto completo (visão, fases, roadmap, decisões de arquitetura), veja [`PLANO_DE_PROJETO.md`](PLANO_DE_PROJETO.md). Para tudo relacionado a dados sensíveis (certificados digitais, CNPJ, dados fiscais) veja [`SEGURANCA.md`](SEGURANCA.md) — **leitura obrigatória antes de mexer em qualquer coisa que toque a fonte de dados real**.

## O que é este projeto

Painel web interno (Go, binário único) que indexa e dá busca/visualização sobre o acervo de documentos de clientes do escritório contábil **FRL** — contratos sociais, CNPJ, certidões, declarações (DEFIS/IRPF), alvarás e afins — hoje espalhados em ~439 pastas de empresas dentro de um backup de rede, sem busca nem controle de acesso.

Está em desenvolvimento solo (usuário + Claude Code), sem equipe. Isso guia toda decisão técnica: preferir simplicidade e manutenibilidade de longo prazo por uma pessoa só, sobre modernidade ou generalização especulativa — mesmo princípio usado em `APP_Contabil_FRL_Clientes`.

**Decisão de linguagem:** Go, não Python. Não é sobre "performance bruta" (o gargalo real é I/O de disco, não CPU) — é sobre entregar um binário único, sem runtime/venv pra instalar na máquina do escritório, com concorrência simples pra varrer ~49 mil arquivos. Ver `PLANO_DE_PROJETO.md` seção 3 para a análise completa.

## Fonte de dados — regra inegociável

A fonte real dos documentos é uma pasta fora deste repositório, num backup local. O caminho é configurável via variável de ambiente `PAINEL_FONTE_DOCUMENTOS`, carregada de um arquivo `.env` local (copiar de `.env.example`, nunca commitado — ver `.gitignore`). **Nunca hardcodar o caminho real no código, em docs ou em qualquer arquivo commitado** — este repositório é público no GitHub; caminho de disco, nome de máquina/usuário e nomes reais de empresa cliente não podem aparecer aqui.

**Este projeto só LÊ dessa pasta. Nunca escreve, move, renomeia ou apaga nada nela.** Nenhuma função em `internal/indexer` ou `internal/web` deve abrir um arquivo dentro da fonte em modo de escrita. Todo metadado extraído vive só no SQLite deste projeto (`data/painel.db`, gitignored).

Levantamento feito em 2026-09-25 (ver `PLANO_DE_PROJETO.md` seção 1 para a íntegra): 439 pastas de primeiro nível, ~48.961 arquivos, ~12 GB, 86% PDF. Decisões de escopo (definitivas, não é "MVP0 provisório"):
- **9 pastas com prefixo `@`** (`@DCTFWEB`, `@IRPF`, `@CERTIFICADOS DIGITAIS`, etc.) são **ignoradas por completo** — não viram empresa, não são percorridas. Juntas somam ~23.900 arquivos (a maioria em `@DCTFWEB` e `@IRPF`), então **o indexer processa ~25 mil arquivos reais de empresa, não os ~49 mil totais** — os números batem, não é bug.
- **`.pfx`/`.p12`** (certificado digital, chave privada) **fora de escopo deste projeto** — o escritório já tem outra aplicação dedicada a isso. O indexer ignora essas extensões, nem o nome do arquivo é catalogado.
- **`.db`/`.dec`/`.rec`/`.frm`/`.dbk`/`.lnk`** — sobra de sistema legado (Domínio Web) ou atalho quebrado, não é documento de cliente. Ignoradas do mesmo jeito.

## Comandos essenciais

```powershell
# build / test (a partir da raiz do repo)
go build ./...
go test ./...

# configuração local (uma vez só) — copiar o exemplo e editar com o caminho real da sua máquina
copy .env.example .env
# depois editar PAINEL_FONTE_DOCUMENTOS dentro de .env

# indexar + extrair texto e sair, sem subir o servidor (lê .env automaticamente)
go run ./cmd/painel -indexar-somente

# indexar + extrair texto + subir o painel web (padrão, sem -indexar-somente)
# fica em http://127.0.0.1:8080 (endereço configurável via PAINEL_ENDERECO no .env)
go run ./cmd/painel

# reindexar só metadados, pulando a extração de texto (mais rápido pra iterar)
go run ./cmd/painel -indexar-somente -pular-busca
```

## Arquitetura

Fluxo em três etapas, cada uma um pacote isolado em `internal/`:

1. **Indexer** (`internal/indexer`) — varre a árvore da fonte (pulando pastas `@...` inteiras), extrai metadados (nome da empresa a partir do nome da pasta, tipo de documento por pasta-pai/nome de arquivo, tamanho, datas), grava em SQLite via upsert idempotente (reindexar não duplica; arquivo que sumiu vira soft-delete, ver `internal/store/documentos.go`). Roda via `go run ./cmd/painel` (com ou sem `-indexar-somente`), nunca em resposta direta a uma requisição HTTP.
2. **Store** (`internal/store`) — única camada que fala com o SQLite (schema, migrations versionadas e embutidas via `embed.FS` em `internal/store/migrations/`, queries, índice FTS5 em `internal/store/busca.go`). Nada fora desse pacote executa SQL direto.
3. **Search** (`internal/search`) — extrai texto de PDF (`github.com/ledongthuc/pdf`, puro Go) e grava no índice FTS5 via `store`. `IndexarPendentes` roda em paralelo (worker pool) só sobre documentos ainda não indexados para busca (`store.DocumentosPdfPendentesDeBusca`) — não reprocessa tudo a cada execução.
4. **Web** (`internal/web`) — handlers HTTP (`net/http` puro, `ServeMux` com padrões de método+path do Go 1.22+), templates server-side embutidos via `embed.FS` (`internal/web/templates/*.html`). Sem SPA, sem build step de JS, sem framework — combina com a filosofia de "binário único, sem dependência de runtime". Ainda sem autenticação (MVP2).

A partir do MVP2 (autenticação + auditoria) entra um quinto pacote, `internal/auth`, responsável por login, sessão e o log de acesso a documentos.

## Estrutura de pastas

```
FRL_Painel_Documentos/
├── cmd/
│   └── painel/                  main.go — ponto de entrada único do binário
├── internal/
│   ├── indexer/                  varredura da fonte + extração de metadados
│   ├── store/                     acesso a SQLite (schema, migrations, queries, FTS5)
│   ├── search/                    extração de texto de PDF + orquestração da indexação de busca
│   ├── web/                       handlers HTTP + templates (embutidos, internal/web/templates/)
│   └── auth/                      login, sessão, log de auditoria (a partir do MVP2 — não existe ainda)
├── testdata/                       fixtures sintéticas para teste (nunca dado real de cliente)
├── data/                            SQLite do projeto (gitignored) — nunca os documentos originais
├── CLAUDE.md
├── PLANO_DE_PROJETO.md
├── SEGURANCA.md
├── README.md
├── go.mod
└── .gitignore
```

Existem hoje (MVP0 concluído): `cmd/painel/`, `internal/indexer/`, `internal/store/` (com `migrations/0001_init.sql`), `testdata/fonte_exemplo/`, além da documentação. `internal/search`, `internal/web`, `internal/auth` e `web/` ainda não existem — nascem no MVP1/MVP2.

## Convenções estabelecidas

- **Idioma dos identificadores**: termos de domínio em **português** (`Empresa`, `Documento`, `TipoDocumento`, `CNPJ`), consistente com `APP_Contabil_FRL_Clientes`. Termos de infraestrutura genérica podem ficar em inglês quando é isso que o ecossistema Go usa (`Server`, `Handler`, `Middleware`, `Router`).
- **Fonte é sempre read-only** (ver seção acima) — regra que vale mais que qualquer outra neste arquivo.
- **Nunca duplicar documentos**: o SQLite guarda só metadados + caminho relativo à fonte. Servir o arquivo original direto (via `http.ServeFile` com o caminho resolvido), nunca copiar PDF pra dentro do repositório ou de `data/`.
- **Testes de integração reais para indexer e store**: sempre que uma função mexe em SQLite ou no sistema de arquivos, testar contra SQLite real (`:memory:` ou arquivo temporário) e uma árvore de arquivos sintética em `testdata/` (algumas pastas de "empresa fictícia" com PDFs de teste pequenos) — nunca testar contra a pasta de backup real. Mesmo padrão de `ContabilFRL.Tests/Infrastructure/`.
- **Sem SPA / sem build step de frontend**: `html/template` + `htmx` quando precisar de interatividade. Justificativa: um binário só, deploy trivial numa máquina do escritório, sem Node/npm no meio.
- **Extensões ignoradas pelo indexer**: `.db`, `.dec`, `.rec`, `.frm`, `.dbk`, `.lnk`, `.pfx`, `.p12` — centralizado em `internal/indexer/regras.go` (`extensoesIgnoradas`), não espalhar essa lista pelo código.
- **Pastas raiz com prefixo `@` são ignoradas por completo** (`internal/indexer/indexer.go`, `prefixoIgnorado`) — nunca viram `Empresa`, o `WalkDir` nem entra nelas.
- **Identificação de documento é por `CaminhoRelativo`**, nunca por `(nome, tamanho)` ou similar — é a única chave estável entre passadas do indexer (nomes de empresa podem ser corrigidos, tamanho de arquivo pode mudar).

## Armadilhas conhecidas (não redescobrir)

1. **`github.com/ledongthuc/pdf` pode dar `panic`, não só devolver `error`, ao processar um PDF real malformado** (descoberto rodando `search.IndexarPendentes` contra os ~24 mil PDFs reais em 2026-09-25 — `panic: loading {5 0}: found {4 0}`, dentro de `Reader.NumPage` → `Value.Key` → `resolve`). Um arquivo problemático não pode derrubar o lote inteiro. **Toda chamada a `ExtrairTexto` dentro de um processamento em lote precisa passar por `recover()`** — já feito em `internal/search/orquestrador.go` (`extrairComRecuperacao`). Se algum código novo chamar `search.ExtrairTexto` fora desse orquestrador (ex: um handler HTTP futuro que extraia sob demanda), replicar o mesmo `recover()`, não assumir que a função só retorna erro normal.
2. **PDFs de CNPJ emitidos pela Receita Federal têm uma fonte com codificação que nenhum extrator de texto decodifica corretamente** (testado com `pdftotext`/xpdf e com `ledongthuc/pdf` — os dois produzem texto ilegível pro mesmo tipo de arquivo). Não é bug deste projeto, é uma limitação do PDF de origem. Efeito prático: a busca por CNPJ via conteúdo de documento não vai funcionar bem justamente no documento onde o CNPJ é mais autoritativo — o nome da pasta/empresa continua sendo o caminho confiável de busca por enquanto.
3. **Chamar um `.exe` da pasta atual sem prefixo (`painel.exe`) falha em `cmd.exe` nesta máquina** — `'painel.exe' não é reconhecido como um comando interno` (validado testando `START.BAT`, 2026-09-25), provavelmente por causa de alguma política de segurança que desativa a busca implícita no diretório atual (`NoDefaultCurrentDirectoryInExePath` ou equivalente). **Sempre referenciar um executável local com `.\` explícito** (`.\painel.exe`, nunca `painel.exe` sozinho) em qualquer script `.bat` deste projeto — não assumir que o comportamento padrão do `cmd.exe` de buscar no diretório atual está sempre ativo.

## Segurança — resumo (ver `SEGURANCA.md` para o checklist completo)

- Roda **só na rede local do escritório**. Nunca exposto à internet, nunca atrás de um túnel/proxy público.
- Certificados digitais (`.pfx`/`.p12`) estão **fora do escopo deste projeto** — outra aplicação do escritório já cuida disso. O indexer nem cataloga o nome desses arquivos.

## Fluxo de trabalho entre sessões (gestão de contexto)

Este projeto é dividido em fases pequenas e sequenciais (ver `PLANO_DE_PROJETO.md` seção 2) exatamente para permitir `/clear` entre uma fase e outra sem perder contexto relevante. Regras:

1. **No início de uma sessão nova** (principalmente logo após `/clear`): ler este arquivo inteiro + a seção "Estado atual" abaixo + a fase corrente em `PLANO_DE_PROJETO.md`. Não é preciso reler o histórico da conversa anterior — o estado real do projeto vive nesses arquivos, não na conversa.
2. **Trabalhar só dentro do escopo da fase corrente.** Se aparecer uma ideia boa fora do escopo, anotar como pendência na fase certa em `PLANO_DE_PROJETO.md` em vez de implementar fora de ordem.
3. **Antes de considerar uma fase encerrada**: atualizar a seção "Estado atual" deste arquivo (o que foi feito, decisões tomadas, testes passando) e marcar a fase como concluída em `PLANO_DE_PROJETO.md`, com data. Só depois disso é seguro rodar `/clear`.
4. Armadilhas novas descobertas no caminho entram na seção "Armadilhas conhecidas" acima, não se perdem numa conversa que vai ser limpa.

## Estado atual (MVP1 — Busca + painel web, implementação concluída em 2026-09-25)

**Fase 0** completa: documentação, `go.mod`, `.gitignore`, `.env`/`.env.example`. Go 1.27.1 instalado.

**MVP0 completo** (indexer de metadados): `internal/store` (schema `empresas`/`documentos`, migrations embutidas, upsert idempotente, soft-delete) + `internal/indexer` (varredura, pula `@...`, ignora certificado/lixo legado, infere `tipo_documento`). Ver histórico de commits pra detalhe — não repetir aqui pra manter este arquivo enxuto.

**MVP1 completo:**
- `internal/search`: `ExtrairTexto` (via `github.com/ledongthuc/pdf`, puro Go) + `IndexarPendentes` (orquestração paralela, incremental, com callback de progresso — ver Armadilha #1 sobre o `recover()` obrigatório).
- `internal/store/busca.go`: índice FTS5 (`documentos_busca`, migration `0002_busca_fts.sql`), `Buscar`, `BuscarEmpresasPorNome`, `ObterDocumento`/`ObterEmpresa`.
- `internal/web`: painel HTTP completo (`/`, `/empresas/{id}`, `/busca`, `/documentos/{id}/arquivo`), templates embutidos, sem autenticação (por enquanto só rede local).
- `cmd/painel`: por padrão indexa metadados + extrai texto + sobe o servidor em `127.0.0.1:8080`. Flags `-indexar-somente` e `-pular-busca` disponíveis.
- **21 testes automatizados passando** (`go test ./...`) em 4 pacotes, incluindo os 5 handlers HTTP principais e uma regressão específica pro panic da lib de PDF (`internal/search/orquestrador_test.go`).
- **Validado de ponta a ponta contra os dados reais** (cópia local): indexação de metadados (430 empresas, 24.042 documentos) + extração de texto de 20.215 PDFs (16.378 com texto, 1.153 sem texto/escaneado, 2.684 erros — não investigado a fundo, não bloqueia) rodando sem crashar, em ~13min30s no total. Servidor subido de verdade e testado via `curl`: lista de empresas, página de empresa com documentos agrupados por tipo, busca por nome e por conteúdo, download de PDF real com `Content-Type` correto — todos funcionando.

**Pendência real:** a extração de texto de 20 mil PDFs demora ~13 minutos numa execução completa (aceitável pra um job batch periódico, mas incremental nas próximas execuções — só reprocessa o que for novo). Os 2.684 erros de extração não foram categorizados um a um; se isso importar no futuro, seria um bom próximo passo de investigação, não um bloqueio de fase.

**Falta pro MVP1 ser considerado 100% fechado:** o usuário abrir o painel no navegador e navegar de verdade (Claude só validou via `curl`, não tem controle de desktop nesta máquina).

**Próximo passo:** MVP2 — autenticação + log de auditoria (ver `PLANO_DE_PROJETO.md` seção 2).
