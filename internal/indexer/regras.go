package indexer

import "strings"

// extensoesIgnoradas nunca viram Documento no banco:
//   - .db/.dec/.rec/.frm/.dbk: sobra de sistema legado (Domínio Web), não é
//     documento de cliente.
//   - .lnk: atalho do Windows, sem valor de arquivo em si.
//   - .pfx/.p12: certificado digital (e-CNPJ/e-CPF) com chave privada — fora
//     de escopo deste projeto, o escritório já tem uma aplicação dedicada
//     para isso. Nem o nome do arquivo é catalogado aqui.
var extensoesIgnoradas = map[string]bool{
	".db":  true,
	".dec": true,
	".rec": true,
	".frm": true,
	".dbk": true,
	".lnk": true,
	".pfx": true,
	".p12": true,
}

func extensaoIgnorada(ext string) bool {
	return extensoesIgnoradas[strings.ToLower(ext)]
}

// regrasPorPastaPai mapeia o nome (maiúsculo) da subpasta imediata de um
// documento para um tipo — usado antes das palavras-chave de nome de
// arquivo, porque a organização em subpastas ("Contrato Social/",
// "Certidões/") é um sinal mais confiável que o nome do arquivo em si.
var regrasPorPastaPai = map[string]string{
	"CONTRATO SOCIAL":  "contrato_social",
	"CERTIDOES":        "certidao",
	"CERTIDÕES":        "certidao",
	"IMPOSTO DE RENDA": "imposto_de_renda",
}

// palavrasChaveArquivo é checada em ordem — a primeira que bater num
// substring do nome do arquivo (maiúsculo) decide o tipo.
var palavrasChaveArquivo = []struct {
	Contem string
	Tipo   string
}{
	{"CONTRATO SOCIAL", "contrato_social"},
	{"CERTIDAO DE BAIXA", "certidao"},
	{"CERTIDÃO DE BAIXA", "certidao"},
	{"CNPJ", "cnpj"},
	{"SINTEGRA", "cartao_sintegra"},
	{"INSCRICAO ESTADUAL", "inscricao_estadual"},
	{"INSCRIÇÃO ESTADUAL", "inscricao_estadual"},
	{"DEFIS", "defis"},
	{"ALVARA", "alvara"},
	{"ALVARÁ", "alvara"},
	{"IRPF", "imposto_de_renda"},
}

// inferirTipoDocumento decide o tipo de um documento a partir do nome da
// subpasta imediata (nomePastaPai) e, se não bater nenhuma regra, do nome
// do próprio arquivo. Retorna "outro" quando nada bate — lista extensível,
// não é pra cobrir 100% dos casos de primeira.
func inferirTipoDocumento(nomePastaPai, nomeArquivo string) string {
	if tipo, ok := regrasPorPastaPai[strings.ToUpper(nomePastaPai)]; ok {
		return tipo
	}

	nomeUpper := strings.ToUpper(nomeArquivo)
	for _, regra := range palavrasChaveArquivo {
		if strings.Contains(nomeUpper, regra.Contem) {
			return regra.Tipo
		}
	}

	return "outro"
}
