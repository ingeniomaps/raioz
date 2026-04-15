const http = require('http');
const port = process.env.PORT || 3000;

const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/html' });
  res.end('<h1>Playground Frontend</h1><p>API: ' + (process.env.API_URL || 'not set') + '</p>');
});

server.listen(port, () => {
  console.log(`Frontend listening on :${port}`);
});
