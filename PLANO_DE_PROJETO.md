# Plano de Projeto — Painel de Documentos FRL

## 0. Visão do produto

O escritório FRL guarda o acervo documental de ~439 empresas clientes (contratos sociais, CNPJ, certidões, DEFIS, IRPF, certificados digitais, fotos) numa estrutura de pastas em backup de rede, sem busca, sem controle de acesso e sem log de quem abriu o quê. O objetivo deste projeto é um **painel web interno** que:

- indexa esse acervo (sem nunca modificá-lo — a pasta original continua sendo a fonte da verdade);
- permite buscar por empresa, CNPJ ou conteúdo do documento;
- exibe os documentos direto no navegador (o PDF renderiza nativamente, sem precisar baixar);
- registra quem acessou o quê e quando — importante porque o acervo inclui certificados digitais (e-CNPJ/e-CPF) e dados fiscais sensíveis;
- sinaliza, no futuro, pendências como certidão desatualizada.

Referência de inspiração (ver conversa de 2026-09-25): arquitetura de indexação read-only do **PhotoPrism** (nunca reorganiza os arquivos originais) + modelo de permissões/auditoria do **Mayan EDMS**. Funcionalidade de OCR/full-text inspirada em **Paperless-ngx** e **Docspell**, mas nenhum código dessas ferramentas é reaproveitado — são só referência de produto.

## 1. Contexto real levantado em 2026-09-25

Levantamento feito diretamente na pasta local de origem dos documentos (caminho real fica só em `.env`, nunca neste arquivo — ver `SEGURANCA.md`):

- **439 pastas de primeiro nível**, a grande maioria = uma empresa cliente (nome da pasta = razão social, ex: `EMPRESA EXEMPLO TRANSPORTES LTDA`). Um punhado de pastas são controle interno do escritório, não cliente: `@CERTIFICADOS DIGITAIS`, `@CONTROLE CND`, `@DCTFWEB`, `@DOCUMENTOS DEP. PESSOAL`, `@IRPF`, `@JURÍDICO` (prefixo `@` parece ser a convenção do próprio escritório pra separar isso — **o indexer deve tratar pastas com prefixo `@` como categoria "controle interno", não como empresa cliente**).
- Dentro de uma pasta de empresa típica: Cartão SINTEGRA, Certidão de Baixa, CNPJ, DEFIS (declaração + recibo, por ano), Imposto de Renda, Inscrição Estadual, Contrato Social, Certidões, fotos, orçamentos avulsos.
- **48.961 arquivos, ~12 GB.** Distribuição por extensão (top): 42.069 PDF (86%), 2.391 `.doc`, 656 `.db`, 626 `.docx`, 610 `.dec`, 601 `.rec`, 363 `.jpg`, 341 `.pfx`, 271 `.frm`, 248 `.dbk`, 142 `.jpeg`, 73 `.xlsx`, 73 `.xls`, 71 `.txt`, 56 `.gif`, 53 `.zip`, 43 `.png`, 37 `.p12`, 20 `.lnk`, 16 `.bmp`.
- **`.pfx`/`.p12` (378 arquivos)**: certificados digitais e-CNPJ/e-CPF — contêm chave privada. Ver `SEGURANCA.md` para o tratamento obrigatório.
- **`.db`/`.dec`/`.rec`/`.frm`/`.dbk` (~2.400 arquivos)**: sobra de um sistema legado (uma pasta menciona "Domínio Web", sistema contábil conhecido no Brasil) — não é documento de cliente, o indexer deve ignorar essas extensões.
- **`.lnk` (20 arquivos)**: atalhos, provavelmente quebrados — ignorar.

## 2. Escopo por fases

Cada fase é pequena o suficiente pra caber numa sessão de trabalho e ser fechada antes de um `/clear` (ver `CLAUDE.md` seção "Fluxo de trabalho entre sessões"). Não pular fase, não misturar escopo de fases diferentes na mesma sessão.

### Fase 0 — Setup do projeto (concluída em 2026-09-25)

Repositório criado, documentação (`CLAUDE.md`, este arquivo, `SEGURANCA.md`, `README.md`), `go.mod`, `.gitignore`. Nenhuma linha de código de produto ainda.

**Critério de avanço:** documentação revisada e aprovada pelo usuário; Go instalado na máquina de desenvolvimento.

### MVP0 — Indexer (não iniciada)

Objetivo: um comando (`go run ./cmd/painel -indexar-somente`) que varre a fonte, popula o SQLite, e pode ser rodado de novo (idempotente — reindexar não duplica registro, atualiza o que mudou e marca como removido o que sumiu da fonte).

Escopo:
- Percorrer recursivamente a fonte (`PAINEL_FONTE_DOCUMENTOS`), tratando cada pasta de primeiro nível como `Empresa` (exceto as com prefixo `@`, que viram categoria "controle interno" — decidir no início desta fase se elas entram no MVP0 ou ficam pra depois, ver seção 9).
- Ignorar as extensões de lixo legado (`.db`, `.dec`, `.rec`, `.frm`, `.dbk`, `.lnk`) — constante centralizada, não espalhada pelo código.
- Para `.pfx`/`.p12`: registrar só nome do arquivo, empresa e caminho — nunca ler o conteúdo binário (ver `SEGURANCA.md`).
- Para os demais arquivos: nome, caminho relativo à fonte, tamanho, data de modificação, tipo de documento inferido do nome (regras simples baseadas em palavras-chave: "CNPJ", "Contrato Social", "Certidão", "DEFIS", "IRPF", "Alvará" — lista extensível).
- Schema SQLite inicial em `internal/store/migrations/` (tabelas `empresas`, `documentos` no mínimo).
- Testes de integração contra SQLite real + fixture sintética em `testdata/` (pastas de "empresa fictícia" criadas só para teste, nunca dado real).

**Critério de avanço:** `go test ./...` passando; rodar o indexer contra a fonte real (só leitura) e confirmar por query manual no SQLite que os números batem aproximadamente com o levantamento da seção 1 (não precisa bater exato — arquivos podem ter mudado desde 2026-09-25).

### MVP1 — Busca + painel web (read-only, sem autenticação) (não iniciada)

Objetivo: interface web local pra listar empresas, buscar e visualizar documentos, ainda sem login (só roda em rede local, ver `SEGURANCA.md`).

Escopo:
- Extração de texto de PDF (decidir a biblioteca/abordagem nesta fase, ver pergunta em aberto na seção 9) + índice FTS5 no SQLite.
- Lista de empresas com busca por nome/CNPJ.
- Página de empresa com os documentos categorizados (pelo tipo inferido no MVP0).
- Visualização de PDF inline (`iframe` + `http.ServeFile` a partir do caminho original — nunca copiar o arquivo).
- Certificados digitais (`.pfx`/`.p12`) aparecem listados (nome, empresa) mas **sem link de download** — isso só chega no MVP2.

**Critério de avanço:** busca funcionando por nome de empresa, CNPJ e conteúdo de PDF; navegação testada manualmente pelo usuário (Claude não tem controle de desktop nesta máquina, só valida via build/testes automatizados).

### MVP2 — Autenticação + log de auditoria (não iniciada)

Objetivo: login simples (usuário/senha, hash no SQLite) e registro de acesso a documento (quem, o quê, quando). É só a partir daqui que `.pfx`/`.p12` passam a ter link de download na interface.

**Critério de avanço:** log de acesso gravando de fato para cada documento aberto, incluindo certificados; sem usuário sem login conseguir chegar em nenhuma página.

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

Ver `SEGURANCA.md` — leitura obrigatória antes do MVP0 começar, porque a regra de "nunca ler conteúdo de `.pfx`/`.p12`" precisa estar no indexer desde a primeira versão, não como retrofit.

## 7. Qualidade e processo de desenvolvimento

- Testes de integração reais (SQLite real + fixtures sintéticas em `testdata/`) para tudo que toca disco ou banco — nunca só mock. Mesmo padrão validado em `APP_Contabil_FRL_Clientes`.
- `go vet` e `gofmt` limpos antes de considerar qualquer fase encerrada.
- Sem framework de UI pesado, sem SPA — decisão de escopo, não só de performance (menos superfície pra manter sozinho).
- Gestão de contexto entre sessões via fases pequenas + `/clear` — ver `CLAUDE.md`.

## 8. Passo a passo imediato

1. Instalar Go na máquina de desenvolvimento (bloqueia MVP0 — ver "Estado atual" em `CLAUDE.md`).
2. Rodar `go mod tidy` depois de instalado, pra confirmar que `go.mod` está correto pra versão real instalada.
3. Começar MVP0 numa sessão dedicada (ideal: logo após `/clear`, lendo só `CLAUDE.md` + esta seção 2 deste arquivo).

## 9. Perguntas em aberto (decidir ao longo do caminho, não bloqueiam o início do MVP0)

- **Extração de texto de PDF:** chamar `pdftotext` (poppler) como processo externo (robusto, mas exige o binário instalado na máquina do escritório) vs. biblioteca pura Go (`ledongthuc/pdf`, `pdfcpu` — sem dependência externa, qualidade de extração a validar). Decidir no início do MVP1, com um teste rápido contra uma amostra real de PDFs da fonte.
- **Pastas com prefixo `@` (controle interno):** entram no MVP0 como uma categoria separada de "empresa", ou ficam de fora até haver um caso de uso claro (ex: V2 de alertas)? Decidir no início do MVP0.
- **Autenticação (MVP2):** usuário/senha simples own-rolled vs. alguma lib de sessão Go padrão. Decidir só ao chegar no MVP2.
- **Deploy:** binário rodando manualmente vs. serviço Windows (`sc create` / NSSM). Decidir quando o MVP1 estiver validado pelo usuário.
