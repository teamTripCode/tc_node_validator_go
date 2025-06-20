@echo off
title Iniciador de Nodos Delegados
color 0A
setlocal enabledelayedexpansion

:: Configuración
set START_PORT=3002
set END_PORT=3023
set PROJECT_DIR=../

:: Animación simple
set SPINNER=|/-\
set COUNT=0

echo ================================================================
echo        🚀 Iniciando Nodos Delegados en Red Blockchain
echo ================================================================
echo.
echo Verificando instalación de Go...
where go >nul 2>&1
if %errorlevel% neq 0 (
    color 0C
    echo ❌ ERROR: Go no está instalado o no está en la variable PATH
    pause
    exit /b 1
)
echo ✅ Go está instalado.
echo.

echo Iniciando nodos delegados desde el puerto %START_PORT% hasta %END_PORT%...
echo Espera mientras se levantan los nodos...

:: Loop para iniciar nodos
for /L %%p in (%START_PORT%,1,%END_PORT%) do (
    set PORT=%%p
    set DATA_DIR=data%%p

    if not exist "!DATA_DIR!" (
        mkdir "!DATA_DIR!"
    )

    set CMD=cd /d "%PROJECT_DIR%" && go run main.go -port=!PORT! -datadir=!DATA_DIR! -verbose=true -seed=localhost:3001

    start "Nodo Delegado !PORT!" cmd /k "!CMD!"
    
    set /a COUNT+=1
    set /a INDEX=!COUNT! %% 4
    call set SPIN=%%SPINNER:~!INDEX!,1%%
    <nul set /p=Iniciando nodo en puerto !PORT! con datos en !DATA_DIR!... !SPIN!        
    timeout /t 1 >nul
    echo.
)

echo.
echo ================================================================
echo ✅ Todos los nodos delegados han sido iniciados correctamente.
echo 🧠 Asegúrate de que el nodo principal (localhost:3001) ya esté corriendo.
echo ================================================================
pause
