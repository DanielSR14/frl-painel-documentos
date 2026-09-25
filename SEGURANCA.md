# Segurança — Painel de Documentos FRL

Este projeto lida com dado fiscal e societário real de clientes (CNPJ, contratos sociais, certidões, declarações). **Certificados digitais (`.pfx`/`.p12`) estão fora do escopo deste projeto por decisão explícita do usuário (2026-09-25)** — o escritório já tem outra aplicação dedicada a isso, e o indexer aqui ignora essas extensões completamente (nem o nome do arquivo é catalogado). Isso remove a classe de risco mais grave (chave privada exposta), mas o que resta ainda é dado sensível o suficiente para tratar com cuidado.

Em caso de dúvida entre "mais funcionalidade" e "mais segurança" neste projeto, a segurança vence — sempre.

## Regra de ouro: nunca exposto à internet

O painel roda **exclusivamente na rede local do escritório**. Nunca:
- atrás de um proxy reverso público, túnel (ngrok, cloudflared, etc.) ou port-forward de roteador;
- em qualquer VPS/cloud pública;
- com uma porta exposta além da rede interna do escritório.

Se algum dia houver necessidade real de acesso remoto (ex: sócio querendo ver de casa), isso é uma decisão de produto explícita a ser tomada com o usuário, com VPN própria do escritório — nunca uma solução implementada "de passagem" numa tarefa de outra fase.

## Certificados digitais e pastas de controle interno — fora de escopo

- **`.pfx`/`.p12`**: extensão ignorada pelo indexer (`internal/indexer/regras.go`, `extensoesIgnoradas`). Nenhum código deste projeto deve voltar a ler, catalogar ou servir esse tipo de arquivo — se um caso de uso futuro precisar disso, é uma decisão de produto nova, não um ajuste incremental.
- **Pastas raiz com prefixo `@`** (`@DCTFWEB`, `@IRPF`, `@CERTIFICADOS DIGITAIS`, etc.): ignoradas por completo pelo indexer (`internal/indexer/indexer.go`, `prefixoIgnorado`) — não é só o conteúdo de certificado, a pasta inteira nem é percorrida.

## Arquivos a nunca indexar como documento de cliente

Extensões identificadas como sobra de sistema legado (não documento): `.db`, `.dec`, `.rec`, `.frm`, `.dbk`, `.lnk`. O indexer ignora essas extensões categoricamente — alguns desses formatos legados podem conter dumps de dados fiscais de forma não controlada, e catalogá-los como "documento navegável" seria expor dado sem entender o que é.

## Fonte é sempre read-only

Repetido aqui porque é crítico: nenhum código deste projeto escreve, move, renomeia ou apaga qualquer coisa dentro da pasta apontada por `PAINEL_FONTE_DOCUMENTOS`. É um backup de produção real de clientes — um bug que apague algo lá não tem "desfazer" garantido.

## O que nunca commitar no git

- `.env` — caminho real da fonte de dados na máquina de desenvolvimento (já gitignored).
- `data/*.db` — o SQLite do projeto pode conter metadado (nomes de empresa, tipos de documento) que não deveria estar num repositório público, mesmo sendo só metadado.
- Qualquer arquivo `.pfx`/`.p12`, mesmo de teste (`.gitignore` já bloqueia por extensão).
- Nomes reais de empresa cliente em qualquer arquivo versionado (código, docs, mensagens de commit) — usar sempre nome fictício em exemplos, como já é a prática em `CLAUDE.md`/`PLANO_DE_PROJETO.md`.
- Dump ou export de dado real de cliente usado "rapidamente" para debugar algo — usar sempre fixture sintética em `testdata/`.

## Log de auditoria (a partir do MVP2)

Registrar no mínimo: usuário, ação (visualizou/baixou), identificador do documento, empresa, timestamp. O log em si é dado sensível (revela quem olhou o quê) — mesma regra de acesso restrito se aplica a ele.

## Checklist de segurança por fase

- [x] Fase 0: este documento existe e foi lido antes de qualquer código ser escrito.
- [x] MVP0: indexer ignora `.pfx`/`.p12` e pastas `@` por completo (coberto por teste automatizado em `internal/indexer/indexer_test.go`, não só revisão manual).
- [x] MVP0: nenhum caminho real de máquina ou nome real de cliente em arquivo versionado (revisado antes do primeiro commit, 2026-09-25).
- [x] MVP1: `/documentos/{id}/arquivo` só serve documentos que existem na tabela `documentos` (nunca um caminho arbitrário — resolvido a partir do `CaminhoRelativo` gravado pelo indexer, com checagem defensiva de path traversal em `internal/web/web.go`).
- [x] MVP1: endereço padrão do servidor é `127.0.0.1:8080` (`cmd/painel/main.go`) — nunca `0.0.0.0` por padrão. Configurável via `PAINEL_ENDERECO` só se alguém decidir mudar conscientemente.
- [x] MVP2: login obrigatório em toda rota que sirva documento (`auth.ExigirLogin` protege tudo exceto `/login`); log de auditoria gravando antes de servir o arquivo, não depois (fail-closed — validado 2026-09-25 conferindo `log_acesso` direto no SQLite após um download real).
- [x] MVP2: senha nunca gravada em texto puro (bcrypt, `internal/auth/auth.go`); senha de bootstrap via `.env` é opcional e nunca sobrescreve usuário existente.
- [x] MVP2: acesso sem sessão válida não gera entrada de auditoria nem serve o arquivo (coberto por teste automatizado, `TestArquivoSemLoginNaoServeENaoRegistraAcesso`).
