# CLAUDE.md — Painel de Documentos FRL

Guia de referência rápida para trabalhar neste repositório. Para o plano de produto completo (visão, fases, roadmap, decisões de arquitetura), veja [`PLANO_DE_PROJETO.md`](PLANO_DE_PROJETO.md). Para tudo relacionado a dados sensíveis (certificados digitais, CNPJ, dados fiscais) veja [`SEGURANCA.md`](SEGURANCA.md) — **leitura obrigatória antes de mexer em qualquer coisa que toque a fonte de dados real**.

## O que é este projeto

Painel web interno (Go, binário único) que indexa e dá busca/visualização sobre o acervo de documentos de clientes do escritório contábil **FRL** — contratos sociais, CNPJ, certidões, declarações (DEFIS/IRPF), alvarás e afins — hoje espalhados em ~439 pastas de empresas dentro de um backup de rede, sem busca nem controle de acesso.

Está em desenvolvimento solo (usuário + Claude Code), sem equipe. Isso guia toda decisão técnica: preferir simplicidade e manutenibilidade de longo prazo por uma pessoa só, sobre modernidade ou generalização especulativa — mesmo princípio usado em `APP_Contabil_FRL_Clientes`.

**Decisão de linguagem:** Go, não Python. Não é sobre "performance bruta" (o gargalo real é I/O de disco, não CPU) — é sobre entregar um binário único, sem runtime/venv pra instalar na máquina do escritório, com concorrência simples pra varrer ~49 mil arquivos. Ver `PLANO_DE_PROJETO.md` seção 3 para a análise completa.

## Fonte de dados — regra inegociável

A fonte real dos documentos é uma pasta fora deste repositório, num backup local. O caminho é configurável via variável de ambiente `PAINEL_FONTE_DOCUMENTOS`, carregada de um arquivo `.env` local (copiar de `.env.example`, nunca commitado — ver `.gitignore`). **Nunca hardcodar o caminho real no código, em docs ou em qualquer arquivo commitado** — este repositório é público no GitHub; caminho de disco, nome de máquina/usuário e nomes reais de empresa cliente não podem aparecer aqui.

**Este projeto só LÊ dessa pasta. Nunca escreve, move, renomeia ou apaga nada nela.** Nenhuma função em `internal/indexer` ou `internal/web` deve abrir um arquivo dentro da fonte em modo de escrita. Todo metadado extraído vive só no SQLite deste projeto (`data/painel.db`, gitignored).

Levantamento feito em 2026-09-25 (ver `PLANO_DE_PROJETO.md` seção 1 para a íntegra): 439 pastas de empresa, ~48.961 arquivos, ~12 GB, 86% PDF. Achados importantes que moldam a arquitetura:
- **378 arquivos `.pfx`/`.p12`** — certificados digitais (e-CNPJ/e-CPF) com chave privada. Tratamento especial obrigatório, ver `SEGURANCA.md`.
- **~2.400 arquivos `.db`/`.dec`/`.rec`/`.frm`/`.dbk`** — sobra de sistema legado (Domínio Web), não é documento de cliente. **Ignorar essas extensões no indexer, nunca catalogar como documento.**
- **20 arquivos `.lnk`** — atalhos quebrados, ignorar também.

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

1. **Indexer** (`internal/indexer`) — varre a árvore da fonte, extrai metadados (nome da empresa a partir do nome da pasta, CNPJ quando aparece no nome do arquivo, tipo de documento por padrão de nome, tamanho, datas), grava em SQLite. Roda sob demanda ou em intervalo (a definir), nunca em resposta direta a uma requisição HTTP.
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

Só existem hoje (Fase 0): os arquivos de documentação, `go.mod`, `.gitignore`. O restante é criado à medida que cada fase do `PLANO_DE_PROJETO.md` é implementada — não criar pacotes vazios adiantado.

## Convenções estabelecidas

- **Idioma dos identificadores**: termos de domínio em **português** (`Empresa`, `Documento`, `TipoDocumento`, `CNPJ`), consistente com `APP_Contabil_FRL_Clientes`. Termos de infraestrutura genérica podem ficar em inglês quando é isso que o ecossistema Go usa (`Server`, `Handler`, `Middleware`, `Router`).
- **Fonte é sempre read-only** (ver seção acima) — regra que vale mais que qualquer outra neste arquivo.
- **Nunca duplicar documentos**: o SQLite guarda só metadados + caminho relativo à fonte. Servir o arquivo original direto (via `http.ServeFile` com o caminho resolvido), nunca copiar PDF pra dentro do repositório ou de `data/`.
- **Testes de integração reais para indexer e store**: sempre que uma função mexe em SQLite ou no sistema de arquivos, testar contra SQLite real (`:memory:` ou arquivo temporário) e uma árvore de arquivos sintética em `testdata/` (algumas pastas de "empresa fictícia" com PDFs de teste pequenos) — nunca testar contra a pasta de backup real. Mesmo padrão de `ContabilFRL.Tests/Infrastructure/`.
- **Sem SPA / sem build step de frontend**: `html/template` + `htmx` quando precisar de interatividade. Justificativa: um binário só, deploy trivial numa máquina do escritório, sem Node/npm no meio.
- **Extensões ignoradas pelo indexer**: `.db`, `.dec`, `.rec`, `.frm`, `.dbk`, `.lnk` (lixo de sistema legado, não documento de cliente) — lista deve ficar centralizada numa constante em `internal/indexer`, não espalhada.

## Armadilhas conhecidas (não redescobrir)

Nenhuma ainda — projeto começando (Fase 0). Preencher aqui conforme bugs reais forem encontrados e corrigidos, sempre com data, igual ao padrão de `APP_Contabil_FRL_Clientes`.

## Segurança — resumo (ver `SEGURANCA.md` para o checklist completo)

- Roda **só na rede local do escritório**. Nunca exposto à internet, nunca atrás de um túnel/proxy público.
- Arquivos `.pfx`/`.p12` (certificado digital): até o MVP2 existir (autenticação + log de auditoria), o indexer só registra a **existência** do arquivo (nome, empresa, caminho) — nunca lê o conteúdo binário, e a interface web nunca oferece link de download para eles. Regra dura, não flexibilizar por conveniência de UI.

## Fluxo de trabalho entre sessões (gestão de contexto)

Este projeto é dividido em fases pequenas e sequenciais (ver `PLANO_DE_PROJETO.md` seção 2) exatamente para permitir `/clear` entre uma fase e outra sem perder contexto relevante. Regras:

1. **No início de uma sessão nova** (principalmente logo após `/clear`): ler este arquivo inteiro + a seção "Estado atual" abaixo + a fase corrente em `PLANO_DE_PROJETO.md`. Não é preciso reler o histórico da conversa anterior — o estado real do projeto vive nesses arquivos, não na conversa.
2. **Trabalhar só dentro do escopo da fase corrente.** Se aparecer uma ideia boa fora do escopo, anotar como pendência na fase certa em `PLANO_DE_PROJETO.md` em vez de implementar fora de ordem.
3. **Antes de considerar uma fase encerrada**: atualizar a seção "Estado atual" deste arquivo (o que foi feito, decisões tomadas, testes passando) e marcar a fase como concluída em `PLANO_DE_PROJETO.md`, com data. Só depois disso é seguro rodar `/clear`.
4. Armadilhas novas descobertas no caminho entram na seção "Armadilhas conhecidas" acima, não se perdem numa conversa que vai ser limpa.

## Estado atual (Fase 0 — Setup, concluída em 2026-09-25)

Repositório criado, ainda sem código. O que existe:
- `CLAUDE.md`, `PLANO_DE_PROJETO.md`, `SEGURANCA.md`, `README.md` — documentação completa e alinhada com o usuário.
- `go.mod` (módulo `frl-painel-documentos`, `go 1.23` — **ajustar a versão para o que for de fato instalado quando o Go for configurado na máquina**).
- `.gitignore` cobrindo binários, SQLite local, certificados e config local.
- Levantamento real da fonte de dados feito e documentado (439 empresas, ~49k arquivos, 12 GB, achados de `.pfx`/`.p12` e lixo legado).

**Pendência que bloqueia o início do MVP0:** Go não está instalado nesta máquina (`go version` falhou em 2026-09-25). Instalar o toolchain antes de começar o indexer.

**Próximo passo:** MVP0 — indexer (ver `PLANO_DE_PROJETO.md` seção 2).
