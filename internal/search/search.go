// Package search extrai texto de PDF para alimentar o índice de busca
// (FTS5, em internal/store). Só lê arquivos — nunca escreve, move ou apaga
// nada na fonte (mesma regra do internal/indexer).
//
// Limitação conhecida (validada em 2026-09-25 contra dados reais): PDFs de
// CNPJ emitidos pela Receita Federal usam uma fonte com codificação que
// nenhuma biblioteca de extração de texto decodifica corretamente (testado
// com xpdf/pdftotext e com esta biblioteca — ambos produzem texto ilegível
// pro mesmo arquivo). PDFs que são só imagem escaneada não têm camada de
// texto alguma. Nos dois casos, ExtrairTexto retorna texto vazio ou um erro
// — não é um bug deste pacote, é uma limitação do documento de origem.
// Cobertura desses casos fica para o OCR da fase V3 (ver PLANO_DE_PROJETO.md).
package search

import (
	"fmt"

	"github.com/ledongthuc/pdf"
)

// ExtrairTexto lê o PDF em caminhoAbsoluto e devolve todo o texto extraído
// da camada de texto do documento. Retorna string vazia (sem erro) quando o
// PDF não tem camada de texto (ex: escaneado como imagem) — não é uma
// condição de erro, é esperado para boa parte do acervo.
func ExtrairTexto(caminhoAbsoluto string) (string, error) {
	f, r, err := pdf.Open(caminhoAbsoluto)
	if err != nil {
		return "", fmt.Errorf("abrir pdf: %w", err)
	}
	defer f.Close()

	leitor, err := r.GetPlainText()
	if err != nil {
		// Comum para PDF só-imagem ou com fonte problemática — tratar como
		// "sem texto", não como erro fatal do processamento em lote.
		return "", nil
	}

	buf := make([]byte, 0, 8192)
	chunk := make([]byte, 8192)
	for {
		n, err := leitor.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf), nil
}
