# Segurança — Painel de Documentos FRL

Este projeto lida com dado mais sensível que a média: não é só documento fiscal de empresa (CNPJ, certidões, IRPF), mas também **certificados digitais com chave privada** (378 arquivos `.pfx`/`.p12` identificados no levantamento de 2026-09-25). Um certificado digital e-CNPJ vazado permite assinar documentos, emitir nota fiscal e acessar sistemas do governo em nome da empresa da vítima — é uma classe de risco bem acima de "documento fiscal exposto".

Este arquivo é a fonte da verdade sobre o que é e não é permitido. Em caso de dúvida entre "mais funcionalidade" e "mais segurança" neste projeto, a segurança vence — sempre.

## Regra de ouro: nunca exposto à internet

O painel roda **exclusivamente na rede local do escritório**. Nunca:
- atrás de um proxy reverso público, túnel (ngrok, cloudflared, etc.) ou port-forward de roteador;
- em qualquer VPS/cloud pública;
- com uma porta exposta além da rede interna do escritório.

Se algum dia houver necessidade real de acesso remoto (ex: sócio querendo ver de casa), isso é uma decisão de produto explícita a ser tomada com o usuário, com VPN própria do escritório — nunca uma solução implementada "de passagem" numa tarefa de outra fase.

## Certificados digitais (`.pfx` / `.p12`) — regra por fase

| Fase | O que é permitido |
|---|---|
| MVP0 (indexer) | Registrar só metadado: nome do arquivo, empresa, caminho relativo, tamanho, data. **Nunca abrir o arquivo para leitura do conteúdo binário.** Não existe motivo técnico pra ler o conteúdo de um `.pfx` só para catalogar sua existência. |
| MVP1 (painel sem login) | O certificado aparece **listado** na página da empresa (nome, data), mas **sem link de download nem preview**. Nenhum handler HTTP deste projeto deve servir o conteúdo de um `.pfx`/`.p12` antes do MVP2. |
| MVP2+ (com login e auditoria) | Download liberado, mas **todo acesso é logado** (usuário, documento, timestamp) antes do arquivo ser servido — logar depois ou de forma best-effort não é aceitável aqui. |

Isso vale mesmo que pareça "só uma tela a mais" implementar o download antes do MVP2 — é a exceção onde não seguir a ordem das fases do `PLANO_DE_PROJETO.md` é inaceitável.

## Arquivos a nunca indexar como documento de cliente

Extensões identificadas como sobra de sistema legado (não documento): `.db`, `.dec`, `.rec`, `.frm`, `.dbk`, `.lnk`. O indexer deve ignorá-las categoricamente — além de não terem valor pro usuário, alguns desses formatos legados podem conter dumps de dados fiscais de forma não controlada, e catalogá-los como "documento navegável" seria expor dado sem entender o que é.

## Fonte é sempre read-only

Repetido aqui porque é crítico: nenhum código deste projeto escreve, move, renomeia ou apaga qualquer coisa dentro da pasta apontada por `PAINEL_FONTE_DOCUMENTOS`. É um backup de produção real de clientes — um bug que apague algo lá não tem "desfazer" garantido.

## O que nunca commitar no git

- Qualquer arquivo `.pfx`/`.p12` real ou de teste (`.gitignore` já bloqueia por extensão, mas revisar `git status`/`git diff` antes de commit sempre que uma fixture de teste for adicionada em `testdata/`).
- `data/*.db` — o SQLite do projeto pode conter metadado (nomes de empresa, CNPJ, tipos de documento) que não deveria estar num repositório, mesmo privado.
- Qualquer arquivo de configuração local com caminho real de máquina, credencial ou segredo (`.env`, `config.local.*`).
- Dump ou export de dado real de cliente usado "rapidamente" para debugar algo — usar sempre fixture sintética em `testdata/`.

## Log de auditoria (a partir do MVP2)

Registrar no mínimo: usuário, ação (visualizou/baixou), identificador do documento, empresa, timestamp. O log em si é dado sensível (revela quem olhou o quê) — mesma regra de acesso restrito se aplica a ele.

## Checklist de segurança por fase

- [x] Fase 0: este documento existe e foi lido antes de qualquer código ser escrito.
- [ ] MVP0: indexer nunca abre `.pfx`/`.p12` para leitura de conteúdo (cobrir com teste automatizado, não só revisão manual).
- [ ] MVP1: nenhuma rota HTTP serve `.pfx`/`.p12` (cobrir com teste de integração que tenta acessar e espera 403/404).
- [ ] MVP1: confirmar que o servidor só faz bind em endereço de rede local (nunca `0.0.0.0` sem revisão consciente, nunca exposto por padrão).
- [ ] MVP2: login obrigatório em toda rota que sirva documento; log de auditoria gravando antes de servir o arquivo, não depois.
