document.addEventListener('DOMContentLoaded', () => {
    const term = new Terminal({
        cursorBlink: true,
        fontSize: 14,
        fontFamily: 'SF Mono, Fira Code, monospace',
        theme: {
            background: '#0a0a0a'
        },
        scrollback: 1000,
        lineHeight: 1.4,
        cols: 120,
        rows: 40
    });

    const terminalElement = document.getElementById('terminal');
    term.open(terminalElement);

    const wsProtocol = location.protocol === 'https:' ? 'wss://' : 'ws://';
    let socket = new WebSocket(`${wsProtocol}${window.location.host}/terminal/ws`);

    const setupSocket = () => {
        socket.onopen = () => {
            console.log('WebSocket connection established');
            term.write('Connected to server.\r\n');
            term.scrollToBottom();
        };

        socket.onmessage = (event) => {
            if (typeof event.data === 'string') {
                term.write(event.data);
            } else {
                const decoder = new TextDecoder('utf-8');
                term.write(decoder.decode(event.data));
            }
            term.scrollToBottom();
        };

        socket.onclose = (event) => {
            if (event.wasClean) {
                console.log(`Connection closed cleanly, code=${event.code} reason=${event.reason}`);
            } else {
                console.log('Connection died');
            }
            term.write('\r\nConnection to the server closed.\r\n');
            document.getElementById('message').innerText = 'Connection to the server closed.';
        };

        socket.onerror = (error) => {
            console.error(`WebSocket error: ${error.message}`);
            term.write(`\r\nWebSocket error: ${error.message}\r\n`);
        };
    };

    setupSocket();

    let inputBuffer = ''; // 入力を蓄積するバッファ
    term.onData((data) => {
        if (socket.readyState === WebSocket.OPEN) {
            socket.send(data);
        }

        inputBuffer += data; // 入力データをバッファに追加

        if (data === '\r') { // エンターキーが押された場合
            const command = inputBuffer.trim(); // 入力コマンドを取得
            if (command === 'exit') {
                document.getElementById('message').innerText = 'You have logged out.';
                socket.close(); // WebSocket を閉じる
            }
            inputBuffer = ''; // バッファをリセット
        }
    });

    window.addEventListener('beforeunload', () => {
        socket.close();
    });

    term.onData(() => {
        term.scrollToBottom();
    });
});
