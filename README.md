# LandWalker

Jogo de sobrevivência em grade com mapa procedural. O servidor, o motor do jogo,
o gerador de mapas e o movimento autônomo do inimigo são implementados em Go.
A interface web é incorporada ao executável, sem depender de Java, Maven, Node
ou conexão com a internet.

## Como executar

Requisito: [Go 1.22 ou mais recente](https://go.dev/dl/).

No diretório do projeto, execute:

```powershell
go run .
```

Abra [http://localhost:3000](http://localhost:3000). Para usar outra porta:

```powershell
$env:PORT = "8080"
go run .
```

## Como jogar

- Use `WASD`, as setas do teclado ou os controles na tela.
- Encontre a porta `⊕` para avançar ao próximo andar. A expedição só termina
  quando a porta do último andar é alcançada; os ursos vivos seguem o jogador.
- As mesmas regras de terreno valem para jogador, urso e gazela: gelo desliza,
  esteiras empurram, portais teleportam, neve exige duas tentativas para entrar
  e buracos cedem na segunda visita.
- Picos altos só podem ser acessados por rampas. A seta da rampa indica o
  sentido da subida; entradas laterais são bloqueadas.
- Em **Custom Grid & Elements**, ajuste mapas de até 200 tiles por andar, de 1
  a 10 andares, a seed, as quantidades exatas de cada tile e a chance de
  perseguição do urso entre 0% e 100%.
- O urso se move em seu próprio intervalo, mesmo quando o jogador fica parado.
  Em cada movimento, a chance configurada decide entre caçar e patrulhar.
- A gazela vaga aleatoriamente. Quando uma entidade não hostil entra no raio
  de três tiles do urso, ele pode escolhê-la como presa.

## Verificação e build

```powershell
go test ./...
go build -o landwalker.exe .
```

O binário gerado contém também a interface web e pode ser iniciado diretamente.

## Estrutura

```text
.
├── main.go               # inicialização e arquivos web incorporados
├── internal/
│   ├── game/             # regras, sessões e ciclo autônomo do inimigo
│   ├── mapgen/           # geração procedural em Go
│   ├── model/            # modelos compartilhados
│   └── server/           # rotas HTTP
└── static/               # interface web
```
