from flask import Flask
app = Flask(__name__)

@app.route('/ping')
def ping():
    return 'pong'

@app.route('/api/health')
def health():
    return {"status": "ok", "service": "flask"}

if __name__ == '__main__':
    app.run(port=5001)
