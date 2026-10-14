$name=$args[0]
$comport=$args[1]
$vmcomport=Get-VMComPort -VMName "$name" -Number "$comport"
$pipe=$vmcomport.Path -replace "\\","/"
socat "EXEC:'$((get-command npiperelay).source -replace "\\","/") -ei $pipe',pty,rawer" "STDIO,escape=0xf,rawer"
