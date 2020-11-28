$ErrorActionPreference = "Stop"

$pipe = $null
$pipeReader = $null
$pipeWriter = $null
$eventPipe = $null
$eventPipeReader = $null

function pglet_event {
    $line = $eventPipeReader.ReadLine()
    Write-Host "Event: $line"
    if ($line -match "(?<target>[^\s]+)\s(?<name>[^\s]+)(\s(?<data>.+))*") {
        return @{
            Target = $Matches["target"]
            Name = $Matches["name"]
            Data = $Matches["data"]
        }
    } else {
        throw "Invalid event data: $line"
    }
}

function pglet_send {
    param (
        $command
    )

    # send command
    $pipeWriter.WriteLine($command)
    $pipeWriter.Flush()

    # parse results
    $OK_RESULT = "ok"
    $ERROR_RESULT = "error"
    
    $result = $pipeReader.ReadLine()

    #Write-Host "Result: $result"

    if ($result -eq $OK_RESULT) {
        return ""
    } elseif ($result.StartsWith("$OK_RESULT ")) {
        return $result.Substring($OK_RESULT.Length + 1)
    } elseif ($result.StartsWith("$ERROR_RESULT ")) {
        throw $result.Substring($ERROR_RESULT.Length + 1)
    } else {
        throw "Unexpected result: $result"
    }
}

try {
    $res = (pglet page page1)

    if ($res -match "(?<pipeName>[^\s]+)\s(?<url>[^\s]+)") {
        $pipeName = $Matches["pipeName"]
        $pageUrl = $Matches["url"]
    } else {
        throw "Invalid event data: $res"
    }

    Write-Host "Page URL: $pageUrl"

    $pipe = new-object System.IO.Pipes.NamedPipeClientStream($pipeName)
    $pipe.Connect(5000)
    $pipeReader = new-object System.IO.StreamReader($pipe)
    $pipeWriter = new-object System.IO.StreamWriter($pipe)
    $pipeWriter.AutoFlush = $true
    
    $eventPipe = new-object System.IO.Pipes.NamedPipeClientStream("$pipeName.events")
    $eventPipe.Connect(5000)
    $eventPipeReader = new-object System.IO.StreamReader($eventPipe)
    
    Start-Sleep -s 10
    pglet_send "clean page"
    Start-Sleep -s 2

    pglet_send "set page title='Hello, world!' gap=10 horizontalAlign=start"
    pglet_send "add text value='Your name' size=large"
    #pglet_send "add button id=submit text=Submit primary=yes event=btn_event"
    #$rowId = pglet_send "add row"
    # pglet_send "add
    #     button id=a1
    #     row
    #         button id=b1"

    pglet_send "add to=page at=0
        stack width=600px horizontalAlign=stretch
          textbox id=fullName value='someone' label=Name placeholder='Your name, please' description='That\'s your name'
          textbox id=bio label='Bio' description='A few words about yourself' value='Line1\nLine2' multiline=true"

    pglet_send "add stack at=0 id=buttons horizontal=true
            button id=submit text=Submit primary=yes event=btn_event
            button id=cancel event=btn_event2"
    
    Start-Sleep -s 2

    pglet_send "add button id=b1 to=buttons"

    pglet_send "set fullName value='John Smith'"
    
    while($true) {
        pglet_event
        $fullName = pglet_send "get fullName value"
        Write-Host "Full name: $fullName"
    }
} catch {
    Write-Host "$_"
} finally {
    $pipe.Close()
    $eventPipe.Close()
}