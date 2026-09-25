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

# rodar o indexer (lê PAINEL_FONTE_DOCUMENTOS do .env automaticamente)
go run ./cmd/painel -indexar-somente
```

## Arquitetura

Fluxo em três etapas, cada uma um pacote isolado em `internal/`:

1. **Indexer** (`internal/indexer`) — varre a árvore da fonte (pulando pastas `@...` inteiras), extrai metadados (nome da empresa a partir do nome da pasta, tipo de documento por pasta-pai/nome de arquivo, tamanho, datas), grava em SQLite via upsert idempotente (reindexar não duplica; arquivo que sumiu vira soft-delete, ver `internal/store/documentos.go`). Roda sob demanda (`-indexar-somente`) ou em intervalo (a definir no MVP1), nunca em resposta direta a uma requisição HTTP.
2. **Store** (`internal/store`) — única camada que fala com o SQLite (schema, migrations manuais versionadas em `internal/store/migrations/`, queries). Nada fora desse pacote executa SQL direto.
3. **Web** (`internal/web`) — handlers HTTP (`net/http` puro ou `chi`, a decidir no MVP1), templates server-side (`html/template`) + `htmx` para interatividade pontual. Sem SPA, sem build step de JS — combina com a filosofia de "binário único, sem dependência de runtime".

A partir do MVP2 (autenticação + auditoria) entra um quarto pacote, `internal/auth`, responsável por login, sessão e o log de acesso a documentos.

## Estrutura de pastas (planejada — pacotes nascem conforme cada fase é implementada)

```
FRL_Painel_Documentos/
├── cmd/
│   └── painel/              main.go — ponto de entrada único do binário
├── internal/
│   ├── indexer/             varredura da fonte + extração de metadados
│   ├── store/                acesso a SQLite (schema, migrations, queries)
│   ├── search/               extração de texto de PDF + índice FTS5 (a partir do MVP1)
│   ├── web/                  handlers HTTP, templates, servir PDF inline
│   └── auth/                 login, sessão, log de auditoria (a partir do MVP2)
├── web/
│   ├── templates/            HTML (html/template)
│   └── static/                CSS/JS estático (sem build step)
├── testdata/                  fixtures sintéticas para teste do indexer (nunca dado real de cliente)
├── data/                       SQLite do projeto (gitignored) — nunca os documentos originais
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

Nenhuma ainda — projeto começando (Fase 0). Preencher aqui conforme bugs reais forem encontrados e corrigidos, sempre com data, igual ao padrão de `APP_Contabil_FRL_Clientes`.

## Segurança — resumo (ver `SEGURANCA.md` para o checklist completo)

- Roda **só na rede local do escritório**. Nunca exposto à internet, nunca atrás de um túnel/proxy público.
- Certificados digitais (`.pfx`/`.p12`) estão **fora do escopo deste projeto** — outra aplicação do escritório já cuida disso. O indexer nem cataloga o nome desses arquivos.

## Fluxo de trabalho entre sessões (gestão de contexto)

Este projeto é dividido em fases pequenas e sequenciais (ver `PLANO_DE_PROJETO.md` seção 2) exatamente para permitir `/clear` entre uma fase e outra sem perder contexto relevante. Regras:

1. **No início de uma sessão nova** (principalmente logo após `/clear`): ler este arquivo inteiro + a seção "Estado atual" abaixo + a fase corrente em `PLANO_DE_PROJETO.md`. Não é preciso reler o histórico da conversa anterior — o estado real do projeto vive nesses arquivos, não na conversa.
2. **Trabalhar só dentro do escopo da fase corrente.** Se aparecer uma ideia boa fora do escopo, anotar como pendência na fase certa em `PLANO_DE_PROJETO.md` em vez de implementar fora de ordem.
3. **Antes de considerar uma fase encerrada**: atualizar a seção "Estado atual" deste arquivo (o que foi feito, decisões tomadas, testes passando) e marcar a fase como concluída em `PLANO_DE_PROJETO.md`, com data. Só depois disso é seguro rodar `/clear`.
4. Armadilhas novas descobertas no caminho entram na seção "Armadilhas conhecidas" acima, não se perdem numa conversa que vai ser limpa.

## Estado atual (MVP0 — Indexer, concluído em 2026-09-25)

**Fase 0** completa: documentação (`CLAUDE.md`, `PLANO_DE_PROJETO.md`, `SEGURANCA.md`, `README.md`), `go.mod`, `.gitignore`, `.env`/`.env.example`. Go 1.27.1 instalado (`go.mod` usa `go 1.25.0`, ajustado automaticamente pelo `go mod tidy`).

**MVP0 completo:**
- `internal/store`: schema inicial (`empresas`, `documentos`) com migration versionada e embutida (`embed.FS`), upsert idempotente por `pasta_relativa`/`caminho_relativo`, soft-delete de documento ausente (`MarcarAusentesComoRemovidos`). Driver `modernc.org/sqlite` (puro Go, sem cgo — mantém o binário único).
- `internal/indexer`: `Run(fonte, store)` varre a fonte, pula pastas `@...` inteiras, ignora extensões de certificado/lixo legado (`internal/indexer/regras.go`), infere `tipo_documento` por pasta-pai e depois por palavra-chave no nome do arquivo (`contrato_social`, `certidao`, `cnpj`, `defis`, `alvara`, `imposto_de_renda`, `cartao_sintegra`, `inscricao_estadual`, ou `outro`).
- `cmd/painel`: binário único, carrega `.env` via `godotenv`, roda a indexação e imprime um resumo. Modo servidor web ainda não existe (fica pro MVP1).
- **8 testes automatizados passando** (`go test ./...`), incluindo integração real contra SQLite (arquivo temporário) e uma árvore de arquivos sintética em `testdata/fonte_exemplo/` (2 empresas fictícias, 1 pasta `@` de teste, 1 `.dbk` e 1 `.pfx` de teste para confirmar que são ignorados). Cobre também idempotência (reindexar não duplica) e detecção de remoção (soft-delete).
- **Validado contra os dados reais** (rodando sobre a cópia local, nunca a pasta original): 430 empresas indexadas, 9 pastas `@` ignoradas, 24.042 documentos indexados, 959 ignorados (lixo legado + certificado). Os ~23.900 arquivos dentro das pastas `@` (majoritariamente `@DCTFWEB` e `@IRPF`) não entram nesse total — decisão de escopo, não bug. Tempo de execução: ~3 min para a árvore inteira.

**Próximo passo:** MVP1 — extração de texto de PDF + índice FTS5 + painel web read-only (ver `PLANO_DE_PROJETO.md` seção 2). Decidir primeiro a biblioteca de extração de texto (seção 9, perguntas em aberto).
