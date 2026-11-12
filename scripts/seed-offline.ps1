param(
    [string]$Target = "all", # all, dictionary, grammar, themes
    [string]$DictPath = "C:\Users\Admin\Desktop\AppTiengAnh\dictionary_data_offline.json",
    [string]$GrammarPath = "C:\Users\Admin\NguPhapTiengAnh\Grammar.json",
    [string]$ThemesPath = "C:\Users\Admin\Desktop\AppTiengAnh\topics.json",
    [string]$DbHost = "localhost",
    [string]$DbPort = "5434",
    [string]$DbUser = "postgres",
    [string]$DbPassword = "23042004",
    [string]$DbName = "knowledgedb",
    [int]$BatchSize = 250
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   EGO KNOWLEDGE OFFLINE SEEDER         " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Target       : $Target" -ForegroundColor Yellow
Write-Host "Dictionary   : $DictPath"
Write-Host "Grammar      : $GrammarPath"
Write-Host "Themes       : $ThemesPath"
Write-Host "Database     : $DbHost`:$DbPort / $DbName (user: $DbUser)"
Write-Host "Batch size   : $BatchSize"
Write-Host "----------------------------------------"

$seedArgs = @(
    "run", "./services/knowledge/cmd/seed",
    "-target", $Target,
    "-dict-path", $DictPath,
    "-grammar-path", $GrammarPath,
    "-themes-path", $ThemesPath,
    "-db-host", $DbHost,
    "-db-port", $DbPort,
    "-db-user", $DbUser,
    "-db-password", $DbPassword,
    "-db-name", $DbName,
    "-batch-size", [string]$BatchSize
)

& go $seedArgs

if ($LASTEXITCODE -eq 0) {
    Write-Host "`n[SUCCESS] Seeding completed successfully!" -ForegroundColor Green
} else {
    Write-Host "`n[ERROR] Seeding failed with exit code $LASTEXITCODE" -ForegroundColor Red
}

