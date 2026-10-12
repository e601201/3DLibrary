@echo off
rem Meta Quest を USB でつなぎ、3DLibrary を VR で見られるようにする。
rem
rem Quest 側の localhost:8766 を、この PC の 127.0.0.1:8766(リモート閲覧の
rem 読み取り専用の口、ADR-0004)へ USB 越しに転送し、Quest のブラウザで開く。
rem WebXR は https か localhost でしか動かないので、Wi-Fi の IP で開くのではなく
rem この転送を使う。アプリは loopback のまま、LAN には何も公開しない。
rem
rem 前提: Quest の開発者モードが有効 / adb(Android SDK Platform-Tools)を導入済み /
rem       3dlibrary.exe が起動している / Quest で「USB デバッグを許可」済み
rem
rem cmd は UTF-8 のバッチを正しく読めない(chcp 65001 でも行が壊れる)ので、
rem このファイルは CP932 で置く(.gitattributes で変換)。
setlocal

set "ADB=adb"
where adb >nul 2>&1
if not errorlevel 1 goto :found
rem winget で入れた直後は PATH が古いままのことがあるので、導入先を直接探す
for /d %%D in ("%LOCALAPPDATA%\Microsoft\WinGet\Packages\Google.PlatformTools_*") do (
  if exist "%%D\platform-tools\adb.exe" set "ADB=%%D\platform-tools\adb.exe"
)
if "%ADB%"=="adb" (
  echo adb が見つかりません。winget install Google.PlatformTools でインストールしてください。
  goto :fail
)
:found

"%ADB%" get-state >nul 2>&1
if errorlevel 1 (
  echo Quest が見つかりません。USB でつなぎ、ヘッドセット内の「USB デバッグを許可」を承認してください。
  goto :fail
)

"%ADB%" reverse tcp:8766 tcp:8766
if errorlevel 1 goto :fail

"%ADB%" shell am start -a android.intent.action.VIEW -d http://localhost:8766 com.oculus.browser >nul
echo Quest のブラウザで http://localhost:8766 を開きました。
echo アセットの詳細画面で「VR で見る」(眼鏡のアイコン)を押してください。
echo ケーブルを抜き差ししたら、このスクリプトをもう一度実行してください。
pause
exit /b 0

:fail
pause
exit /b 1
