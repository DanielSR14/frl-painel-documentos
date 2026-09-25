# Plano de Projeto — Painel de Documentos FRL

## 0. Visão do produto

O escritório FRL guarda o acervo documental de ~439 empresas clientes (contratos sociais, CNPJ, certidões, DEFIS, IRPF, certificados digitais, fotos) numa estrutura de pastas em backup de rede, sem busca, sem controle de acesso e sem log de quem abriu o quê. O objetivo deste projeto é um **painel web interno** que:

- indexa esse acervo (sem nunca modificá-lo — a pasta original continua sendo a fonte da verdade);
- permite buscar por empresa, CNPJ ou conteúdo do documento;
- exibe os documentos direto no navegador (o PDF renderiza nativamente, sem precisar baixar);
- registra quem acessou o quê e quando — importante porque o acervo inclui dados fiscais sensíveis;
- sinaliza, no futuro, pendências como certidão desatualizada.

**Fora de escopo, por decisão explícita do usuário (2026-09-25):** certificados digitais (`.pfx`/`.p12`) — já existe outra aplicação do escritório dedicada a isso — e as pastas de controle interno com prefixo `@` (`@DCTFWEB`, `@IRPF`, etc.), que não são organizadas por empresa.

Referência de inspiração (ver conversa de 2026-09-25): arquitetura de indexação read-only do **PhotoPrism** (nunca reorganiza os arquivos originais) + modelo de permissões/auditoria do **Mayan EDMS**. Funcionalidade de OCR/full-text inspirada em **Paperless-ngx** e **Docspell**, mas nenhum código dessas ferramentas é reaproveitado — são só referência de produto.

## 1. Contexto real levantado em 2026-09-25

Levantamento feito diretamente na pasta local de origem dos documentos (caminho real fica só em `.env`, nunca neste arquivo — ver `SEGURANCA.md`):

- **439 pastas de primeiro nível**, a grande maioria = uma empresa cliente (nome da pasta = razão social, ex: `EMPRESA EXEMPLO TRANSPORTES LTDA`). **9 pastas** são controle interno do escritório, não cliente: `@CERTIFICADOS DIGITAIS`, `@CONTROLE CND`, `@DCTFWEB`, `@DOCUMENTOS DEP. PESSOAL`, `@IRPF`, `@JURÍDICO` e mais 3 (prefixo `@`). **Decisão final: essas 9 pastas são ignoradas por completo pelo indexer** — juntas somam ~23.900 arquivos (a maioria em `@DCTFWEB`, ~16.900, e `@IRPF`, ~5.900), então o indexer processa de fato ~25 mil arquivos, não os ~49 mil totais.
- Dentro de uma pasta de empresa típica: Cartão SINTEGRA, Certidão de Baixa, CNPJ, DEFIS (declaração + recibo, por ano), Imposto de Renda, Inscrição Estadual, Contrato Social, Certidões, fotos, orçamentos avulsos.
- **48.961 arquivos, ~12 GB.** Distribuição por extensão (top): 42.069 PDF (86%), 2.391 `.doc`, 656 `.db`, 626 `.docx`, 610 `.dec`, 601 `.rec`, 363 `.jpg`, 341 `.pfx`, 271 `.frm`, 248 `.dbk`, 142 `.jpeg`, 73 `.xlsx`, 73 `.xls`, 71 `.txt`, 56 `.gif`, 53 `.zip`, 43 `.png`, 37 `.p12`, 20 `.lnk`, 16 `.bmp`.
- **`.pfx`/`.p12` (378 arquivos)**: certificados digitais e-CNPJ/e-CPF. **Decisão final: fora de escopo deste projeto** — o escritório já tem outra aplicação dedicada a isso. O indexer ignora essas extensões completamente.
- **`.db`/`.dec`/`.rec`/`.frm`/`.dbk` (~2.400 arquivos)**: sobra de um sistema legado (uma pasta menciona "Domínio Web", sistema contábil conhecido no Brasil) — não é documento de cliente, o indexer ignora essas extensões.
- **`.lnk` (20 arquivos)**: atalhos, provavelmente quebrados — ignorados também.

## 2. Escopo por fases

Cada fase é pequena o suficiente pra caber numa sessão de trabalho e ser fechada antes de um `/clear` (ver `CLAUDE.md` seção "Fluxo de trabalho entre sessões"). Não pular fase, não misturar escopo de fases diferentes na mesma sessão.

### Fase 0 — Setup do projeto (concluída em 2026-09-25)

Repositório criado, documentação (`CLAUDE.md`, este arquivo, `SEGURANCA.md`, `README.md`), `go.mod`, `.gitignore`. Nenhuma linha de código de produto ainda.

**Critério de avanço:** documentação revisada e aprovada pelo usuário; Go instalado na máquina de desenvolvimento.

### MVP0 — Indexer (concluída em 2026-09-25)

Objetivo: um comando (`go run ./cmd/painel -indexar-somente`) que varre a fonte, popula o SQLite, e pode ser rodado de novo (idempotente — reindexar não duplica registro, atualiza o que mudou e marca como removido o que sumiu da fonte).

Escopo implementado:
- Percorrer recursivamente a fonte (`PAINEL_FONTE_DOCUMENTOS`), tratando cada pasta de primeiro nível como `Empresa`, **pulando por completo** as com prefixo `@` (não viram empresa, não são percorridas — `internal/indexer/indexer.go`).
- Ignorar as extensões de lixo legado e certificado digital (`.db`, `.dec`, `.rec`, `.frm`, `.dbk`, `.lnk`, `.pfx`, `.p12`) — constante centralizada em `internal/indexer/regras.go`, não espalhada pelo código.
- Para os demais arquivos: nome, caminho relativo à fonte, tamanho, data de modificação, tipo de documento inferido primeiro pela subpasta imediata e depois por palavra-chave no nome do arquivo (`contrato_social`, `certidao`, `cnpj`, `cartao_sintegra`, `inscricao_estadual`, `defis`, `alvara`, `imposto_de_renda`, ou `outro`).
- Schema SQLite inicial em `internal/store/migrations/0001_init.sql` (tabelas `empresas`, `documentos`), aplicado via migration runner embutido (`embed.FS`).
- 8 testes de integração contra SQLite real (arquivo temporário) + fixture sintética em `testdata/fonte_exemplo/`, cobrindo indexação inicial, idempotência e detecção de remoção (soft-delete).

**Critério de avanço — atingido:** `go test ./...` passando; indexer rodado contra a cópia local dos dados reais (nunca a fonte original): 430 empresas, 9 pastas `@` ignoradas, 24.042 documentos indexados, 959 ignorados — bate com o levantamento da seção 1 uma vez descontados os ~23.900 arquivos dentro das pastas `@`.

### MVP1 — Busca + painel web (read-only, sem autenticação) (implementação concluída em 2026-09-25, validação manual do usuário pendente)

Objetivo: interface web local pra listar empresas, buscar e visualizar documentos, ainda sem login (só roda em rede local, ver `SEGURANCA.md`).

Escopo implementado:
- Extração de texto de PDF via `github.com/ledongthuc/pdf` (puro Go, decisão final — ver seção 9) + índice FTS5 no SQLite (`internal/store/busca.go`). Processamento paralelo e incremental (`internal/search.IndexarPendentes`, só processa PDF ainda sem entrada no índice).
- `GET /` — lista de empresas.
- `GET /empresas/{id}` — documentos da empresa, agrupados por tipo.
- `GET /busca?q=` — busca por nome de empresa (`LIKE`) e por conteúdo de documento (frase literal via FTS5), com trecho de contexto.
- `GET /documentos/{id}/arquivo` — serve o arquivo original inline (`http.ServeFile`, com checagem defensiva de path traversal), nunca copia o arquivo.
- 5 testes de integração (`internal/web/web_test.go`) cobrindo listagem, detalhe, 404, busca por conteúdo e download.

**Limitação conhecida, não é bug:** busca por CNPJ via conteúdo não funciona bem porque o PDF de CNPJ da Receita Federal tem uma fonte com codificação que nenhum extrator decodifica (ver `CLAUDE.md`, Armadilhas conhecidas #2). Busca por nome de empresa continua sendo o caminho confiável.

**Critério de avanço:** `go test ./...` passando (feito); indexação completa rodada contra a cópia local dos dados reais sem crashar (feito, após corrigir a Armadilha #1 — panic da lib de PDF); **falta**: o usuário abrir o painel no navegador e navegar de verdade (Claude não tem controle de desktop nesta máquina).

### MVP2 — Autenticação + log de auditoria (implementação concluída em 2026-09-25)

Objetivo: login simples (usuário/senha, hash no SQLite) e registro de acesso a documento (quem, o quê, quando).

Escopo implementado:
- Login usuário/senha (bcrypt), sessão via cookie `HttpOnly` (token opaco, 12h fixas, sem renovação — decisão de simplicidade, ver seção 9).
- Todas as rotas exceto `/login` atrás de `auth.ExigirLogin`; servidor recusa subir sem nenhum usuário cadastrado.
- `log_acesso` gravado **antes** de servir qualquer documento, fail-closed (se o registro falhar, o arquivo não é servido).
- Gestão de usuário via CLI (`-criar-usuario`, `-alterar-senha`, senha oculta com confirmação) — sem tela de administração de usuários na web (decisão de escopo: só o dono do escritório cria/gerencia contas, não precisa de UI pra isso ainda).

**Critério de avanço — atingido:** 37 testes passando; validado com o servidor real rodando (login errado rejeitado, login correto aceito, download com log correto no SQLite, acesso sem sessão bloqueado e sem gerar log de auditoria, logout invalidando a sessão).

### V2 — Alertas e regras de negócio (não iniciada, escopo a refinar)

Ideia inicial: cruzar com as pastas de controle interno (`@CONTROLE CND`, por exemplo) pra sinalizar certidão/CND desatualizada. Escopo real a definir quando chegar nessa fase — não detalhar agora pra não desperdiçar contexto com algo que pode mudar.

### V3 — OCR para PDFs sem texto (não iniciada, escopo a refinar)

PDFs que são só imagem escaneada (sem camada de texto) não entram na busca do MVP1. Avaliar Tesseract (via `gosseract` ou chamando o binário) só se isso se mostrar um problema real depois do MVP1 rodando — não adiantar esse trabalho.

## 3. Stack técnica

- **Linguagem:** Go. Decisão do usuário, reforçada aqui: o gargalo do problema (varrer ~49 mil arquivos e servir PDF) é I/O, não CPU — Go entrega binário único, sem runtime pra instalar, concorrência simples (goroutines) pro indexer, e servidor HTTP embutido na standard library. C puro foi descartado por custo de desenvolvimento (reimplementar HTTP/JSON/etc. do zero sem ganho de performance real pra este caso).
- **Banco:** SQLite (`modernc.org/sqlite` — driver puro Go, sem cgo, mantém o binário único sem dependência de toolchain C no build) com FTS5 para busca full-text.
- **Web:** `net/http` da standard library (avaliar `chi` só se o roteamento nu ficar repetitivo) + `html/template` + `htmx` para interatividade pontual. Sem SPA, sem Node/npm.
- **Extração de texto de PDF:** decisão adiada para o início do MVP1 (ver seção 9).

## 4. Arquitetura de software

Três camadas isoladas em pacotes (`internal/indexer`, `internal/store`, `internal/web`), detalhado em `CLAUDE.md` seção "Arquitetura". Regra de dependência: `web` e `indexer` dependem de `store`; `store` não depende de nenhum dos outros dois. Nenhum pacote fora de `internal/store` executa SQL diretamente.

## 5. Estrutura de pastas

Ver `CLAUDE.md` seção "Estrutura de pastas" — mantida num único lugar para não divergir.

## 6. Segurança e conformidade

Ver `SEGURANCA.md`. Certificados digitais estão fora de escopo (ver seção 0) — a superfície de risco que resta é o acervo fiscal/societário em si (CNPJ, contratos sociais, certidões), tratado com acesso restrito à rede local e, a partir do MVP2, log de auditoria.

## 7. Qualidade e processo de desenvolvimento

- Testes de integração reais (SQLite real + fixtures sintéticas em `testdata/`) para tudo que toca disco ou banco — nunca só mock. Mesmo padrão validado em `APP_Contabil_FRL_Clientes`.
- `go vet` e `gofmt` limpos antes de considerar qualquer fase encerrada.
- Sem framework de UI pesado, sem SPA — decisão de escopo, não só de performance (menos superfície pra manter sozinho).
- Gestão de contexto entre sessões via fases pequenas + `/clear` — ver `CLAUDE.md`.

## 8. Passo a passo imediato

1. ~~Instalar Go na máquina de desenvolvimento.~~ Feito (Go 1.27.1, 2026-09-25).
2. ~~Começar MVP0.~~ Feito e validado contra dados reais (ver "Estado atual" em `CLAUDE.md`).
3. ~~Começar MVP1.~~ Feito e validado (busca + painel web).
4. ~~Começar MVP2.~~ Feito e validado (autenticação + auditoria).
5. Começar V2 (alertas) numa sessão dedicada — primeiro passo é definir o escopo real (ver seção 2), hoje só existe como ideia.

## 9. Perguntas em aberto (decidir ao longo do caminho)

- ~~**Extração de texto de PDF:**~~ **Decidido em 2026-09-25: `github.com/ledongthuc/pdf`** (puro Go). Testado empiricamente contra amostras reais junto com `pdftotext` (xpdf, o único disponível nesta máquina — vem do Git for Windows, não seria garantido existir na máquina de produção do escritório): os dois extraem texto perfeitamente de PDF "normal" (ex: recibo DEFIS, com acentuação correta), e os dois falham igualmente no PDF de CNPJ da Receita Federal (fonte com codificação quebrada — não é diferença de qualidade entre as ferramentas). Como o resultado é equivalente, venceu a opção sem dependência de binário externo, mantendo o binário único.
- ~~**Autenticação (MVP2):**~~ **Decidido em 2026-09-25: usuário/senha own-rolled** (bcrypt + sessão em cookie `HttpOnly` com token opaco validado contra o SQLite, sem lib de sessão externa). Simples o suficiente pro tamanho do projeto; gestão de usuário só via CLI, sem tela de administração — reavaliar só se o número de usuários crescer muito.
- **Duração de sessão (12h fixas, sem renovação):** decisão simples pro MVP2. Se no uso real isso incomodar (forçar login toda manhã), considerar renovação automática enquanto o usuário estiver ativo — não implementar preventivamente.
- **Deploy:** binário rodando manualmente (via `START.BAT`) vs. serviço Windows (`sc create` / NSSM, pra já subir com o Windows). Decidir quando o usuário validar o uso diário do painel.
