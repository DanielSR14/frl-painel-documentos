@echo off
setlocal
cd /d "%~dp0"

if not exist ".env" (
    echo Arquivo .env nao encontrado. Copiando .env.example para .env...
    copy /Y ".env.example" ".env" >nul
    echo.
    echo IMPORTANTE: edite o .env e ajuste PAINEL_FONTE_DOCUMENTOS para o
    echo caminho real da pasta de documentos antes de continuar.
    echo.
    notepad ".env"
    echo.
    echo Pressione qualquer tecla depois de salvar o .env para continuar...
    pause >nul
)

echo.
echo Compilando...
go build -o painel.exe .\cmd\painel
if errorlevel 1 (
    echo.
    echo ERRO: falha ao compilar. Verifique se o Go esta instalado ^(go version^)
    echo e se PATH inclui o Go ^(pode ser preciso abrir um terminal novo apos instalar^).
    pause
    exit /b 1
)

echo.
echo Iniciando o Painel de Documentos FRL...
echo Na primeira vez, ou depois de mudancas grandes no acervo, a indexacao
echo pode levar varios minutos antes do painel ficar disponivel.
echo.
echo Para parar o servidor, feche esta janela ou pressione Ctrl+C.
echo.
.\painel.exe

echo.
echo O servidor foi encerrado.
pause
