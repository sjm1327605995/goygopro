Get-CimInstance Win32_Process -Filter "Name='chrome.exe'" |
  Where-Object { $_.CommandLine -like '*chrome-devtools-mcp*' } |
  ForEach-Object { Write-Output ("PID=" + $_.ProcessId) }
