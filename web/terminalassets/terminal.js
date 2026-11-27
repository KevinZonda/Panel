(() => {
  const screen = document.getElementById('terminal-screen');
  const status = document.getElementById('terminal-status');
  const connectButton = document.getElementById('terminal-connect');
  const disconnectButton = document.getElementById('terminal-disconnect');
  const term = new Terminal({
    disableStdin: true,
    cursorBlink: true,
    scrollback: 2000,
    theme: { background: '#111827', foreground: '#e5e7eb' },
  });
  const fit = new FitAddon.FitAddon();
  const encoder = new TextEncoder();
  const maxBufferedBytes = 2 * 1024 * 1024;
  let session;

  term.loadAddon(fit);
  term.open(screen);
  new ResizeObserver(() => fit.fit()).observe(screen);

  function sizeMessage() {
    return JSON.stringify({ cols: term.cols, rows: term.rows });
  }

  function sendInput(bytes) {
    if (!session?.ready || session.socket.readyState !== WebSocket.OPEN) return;
    const socket = session.socket;
    if (socket.bufferedAmount + bytes.length > maxBufferedBytes) {
      socket.close(4000, 'Terminal input buffer is full');
      return;
    }
    for (let offset = 0; offset < bytes.length; offset += 16384) {
      socket.send(bytes.subarray(offset, offset + 16384));
    }
  }

  term.onData((data) => sendInput(encoder.encode(data)));
  term.onBinary((data) => sendInput(Uint8Array.from(data, (byte) => byte.charCodeAt(0))));
  term.onResize(() => {
    if (session?.ready && session.socket.readyState === WebSocket.OPEN) {
      session.socket.send(sizeMessage());
    }
  });

  connectButton.onclick = () => {
    fit.fit();
    const url = new URL('/ui/terminal/ws', window.location.href);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    const socket = new WebSocket(url);
    socket.binaryType = 'arraybuffer';
    const current = { socket, ready: false, pendingBytes: 0 };
    session = current;
    connectButton.disabled = true;
    disconnectButton.disabled = false;
    status.textContent = 'Connecting…';

    socket.onopen = () => socket.send(sizeMessage());
    socket.onmessage = ({ data }) => {
      if (data === 'ready') {
        term.reset();
        current.ready = true;
        term.options.disableStdin = false;
        status.textContent = 'Connected';
        socket.send(sizeMessage());
        term.focus();
      } else if (data instanceof ArrayBuffer) {
        current.pendingBytes += data.byteLength;
        if (current.pendingBytes > maxBufferedBytes) {
          socket.close(4000, 'Terminal output buffer is full');
          return;
        }
        term.write(new Uint8Array(data), () => { current.pendingBytes -= data.byteLength; });
      }
    };

    socket.onclose = ({ code, reason }) => {
      session = undefined;
      term.options.disableStdin = true;
      connectButton.disabled = false;
      disconnectButton.disabled = true;
      status.textContent = reason || (current.ready || code === 1000 ? 'Disconnected' : 'Connection failed');
    };
  };

  disconnectButton.onclick = () => session?.socket.close(1000, 'Disconnected');
  window.addEventListener('pagehide', () => session?.socket.close());
})();
