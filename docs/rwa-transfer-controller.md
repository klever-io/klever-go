# Transfer Controller para tokens KDA (suporte a RWA)

## 1. Visão geral

Introduz um campo opcional `ControllerAddress` no KDA. Quando preenchido, o protocolo passa a restringir transferências do ativo: toda transferência precisa ter o endereço do controller como remetente ou como destinatário (sob condições específicas). Isso permite que um contrato — chamado de "controller" — seja o ponto único de aplicação de regras de negócio sobre o ativo (KYC, whitelist, vesting, lockup, limites regulatórios etc.).

O campo é **write-once**: pode ser definido vazio na criação e gravado posteriormente uma única vez via `AssetTrigger SetController`; depois disso, fica permanentemente bloqueado. Quando vazio (padrão ou ainda não gravado), o comportamento atual de transferências do KDA é preservado.

## 2. Motivação

Estamos preparando o protocolo para suportar tokens RWA (Real-World Assets). Esses ativos exigem que toda movimentação seja mediada pela lógica de um contrato — sem essa mediação, não é possível garantir as restrições impostas pelo emissor ou pelo regulador.

Hoje, qualquer detentor pode emitir um `TransferContract` direto e movimentar o saldo sem passar pelo contrato. Mesmo que o token só viva dentro de um ecossistema de smart contracts, basta o usuário ter saldo direto para a regra ser furada. Precisamos de uma garantia em nível de protocolo de que o contrato controller está sempre na rota da transferência.

## 3. Requisitos funcionais

| # | Requisito |
|---|---|
| F1 | KDAs podem ser criados com um `ControllerAddress` opcional. |
| F2 | `ControllerAddress` pode ser gravado posteriormente uma única vez via `AssetTrigger SetController` (write-once). Após gravado, é imutável. |
| F3 | Quando `ControllerAddress` é vazio, comportamento atual é preservado (compatibilidade retroativa). |
| F4 | Quando `ControllerAddress` é preenchido, transferências do ativo que não envolvem o controller como uma das pontas são rejeitadas. |
| F5 | A rejeição ocorre em tempo de execução (cobra fee, emite receipt de erro, registra na blockchain). |
| F6 | A regra vale para todos os tipos de KDA (Fungible / SemiFungible / NonFungible). |
| F7 | A regra se aplica somente a ativos KDA — KLV (token nativo) está fora de escopo. |
| F8 | Apenas o `OwnerAddress` do ativo pode invocar `SetController`. |

## 4. Decisões de design

O design final foi convergido em três iterações com base no caso de uso (contrato RWA). Cada decisão registrada abaixo tem alternativas que foram descartadas explicitamente.

### 4.1 Granularidade: token-level, não account-level

**Decisão:** propriedade vive no KDA, não em uma permissão de conta.

**Alternativas descartadas:**
- *Permissão de conta:* exigiria configuração por conta de cada usuário detentor, inviável em escala e contra o modelo RWA (regra é do ativo, não do dono).
- *Regra global de rede:* genérica demais; afetaria todos os tokens.

### 4.2 Modelo de autoridade: controller único designado

**Decisão:** o KDA carrega o endereço de exatamente um contrato controller. Apenas operações que tenham esse endereço como uma das pontas da transferência são permitidas.

**Alternativas descartadas:**
- *"Qualquer smart contract":* permite que um usuário implante um SC pass-through e burle as restrições do RWA chamando `kdaTransfer` a partir do seu próprio contrato. Inseguro.
- *Reaproveitar `OwnerAddress` / `AdminAddress`:* esses endereços têm significados administrativos (criação, alteração de metadados, papéis). Sobrecarregá-los com "quem pode mover o ativo" mistura responsabilidades e dificulta auditoria.

### 4.3 Custódia: saldos reais nos usuários

**Decisão:** os tokens ficam nos saldos dos usuários, e o controller participa de toda movimentação como uma das pontas.

**Alternativa descartada:**
- *Custódia no contrato (modelo "shares"):* o contrato seria o único detentor e usuários teriam representações internas. Simplificaria a regra ("qualquer SC pode mover"), mas reduz a UX, dificulta exibição de saldo em wallets e integrações, e impede que o token seja transacionado por built-ins padrão (`kdaTransfer`).

### 4.4 Camada de aplicação: `accounts.Transfer`

**Decisão:** a verificação acontece em `core/kapp/accounts/accounts.go::Transfer`, logo após o check de `IsPaused`.

**Por quê:** `accounts.Transfer` é o ponto de convergência de *todos* os caminhos de transferência:

```
1. TransferContract direto (EOA)         → txProcess.transferContract → accounts.Transfer
2. SC chamando kdaTransfer builtin       → kleverTransfer.performKDATransfer → accounts.Transfer
3. SC chamando BlockChainHook            → BlockChainHookImpl.KDATransfer → accounts.Transfer
4. Valor anexado em chamada para SC      → smartContract/process.go → accounts.Transfer
5. SC chamando outro SC com valor        → vmhost/hostCore/execution.go → accounts.Transfer
```

Validar em qualquer camada acima exigiria duplicar a checagem; validar em `accounts.Transfer` cobre todos os caminhos com uma única regra.

**Alternativas descartadas:**
- *Validação na intercepted-tx (antes da execução):* não cobre os casos 2–5, que são gerados dinamicamente durante execução. Além disso, o requisito é rejeitar em execução para que a falha cobre fee e emita receipt — não na fase de intercepção.

### 4.5 Discriminador: `(sender, recipient, cType)` em vez de "estou dentro de um SC?"

**Decisão:** a regra é avaliada com base em três sinais que já chegam em `accounts.Transfer`:
- `sender` — endereço de origem
- `tc.ToAddress` — endereço de destino
- `cType` — tipo do contrato (TransferContract vs SmartContract)

**Por quê:** uma primeira proposta usava um contador "estou dentro de um SC?" no `KAppContext`, incrementado/decrementado nas entradas e saídas da VM. Foi descartado por dois motivos:

1. **Não é discriminante suficiente.** "Estou dentro de um SC" responde "esta transferência foi disparada por um SC?", mas a pergunta que importa para RWA é "esta transferência envolve o contrato controller?". São perguntas diferentes — um usuário pode invocar um SC qualquer e a regra precisa rejeitar mesmo "dentro de um SC".
2. **Plumbing invasivo.** Exigiria modificar `vmhost`, `BlockChainHookImpl`, `smartContract/process`, e qualquer caminho futuro que entre em execução de VM. Os três sinais que já temos respondem à pergunta correta sem nova infraestrutura.

### 4.6 Mutabilidade: write-once

**Decisão:** `ControllerAddress` pode ser definido em `CreateAsset` ou gravado depois uma única vez via `AssetTrigger SetController`. Após gravado, é permanente.

**Por quê:** o desenho original era "imutável em criação", mas há uma ordem natural de operação que isso quebra:

1. O `assetID` é derivado de dados do bloco e do `txNonce` da criação — não existe antes do `CreateAsset`.
2. O contrato controller normalmente é parametrizado com o `assetID` que vai controlar.
3. Logo, o fluxo prático é: **criar o ativo → deployar o controller (configurado com o `assetID`) → gravar o `ControllerAddress` no ativo**.

Write-once concilia as duas necessidades: o ativo pode ser criado sem controller, o controller é deployado conhecendo o `assetID`, e em seguida o owner faz a amarração — depois disso a regra fica gravada para sempre, mantendo a garantia "sem rotação, sem rugpull" para os detentores.

**Janela de risco entre criação e gravação:** enquanto `ControllerAddress` estiver vazio, transferências ocorrem normalmente. O owner deve evitar distribuir supply antes de gravar o controller. Adicionar um "awaiting flag" que bloqueasse transferências nesse intervalo foi considerado e descartado (mais código, e o problema é facilmente evitado mantendo o supply no owner até a amarração).

**Autoridade:** apenas o `OwnerAddress` pode invocar `SetController`. `AdminAddress` foi descartado porque administradores típicos cuidam de metadados e papéis — entregar a configuração do gate de transferência seria escopo demais para esse perfil.

**Alternativas descartadas:**
- *Imutável só em criação:* incompatível com a ordem de deploy descrita acima.
- *Mutável livremente via `AssetTrigger`:* introduziria risco de captura do ativo após emissão.
- *Mutabilidade controlada por flag `CanChangeController`:* aceita, mas adicionaria complexidade sem demanda real — RWAs querem garantia forte; emissores que quiserem flexibilidade podem implementá-la na camada SC com um proxy.

### 4.7 Nome do campo: `ControllerAddress`

**Decisão:** `ControllerAddress` em `KDAData`, posicionado junto a `OwnerAddress` e `AdminAddress`.

**Alternativas descartadas:** `TransferController`, `TransferAuthority`. O sufixo `Address` espelha o estilo dos campos vizinhos e deixa explícito o tipo do valor (endereço, não bool).

## 5. Modelo de dados

### 5.1 KDAData (`kapps/proto/kda.proto`)

```proto
message KDAData {
  // ... campos existentes ...
  bytes AdminAddress       = 19 [json_name = "adminAddress"];
  bytes ControllerAddress  = 20 [json_name = "controllerAddress"];
}
```

### 5.2 CreateAssetContract (`data/transaction/proto/contracts.proto`)

```proto
message CreateAssetContract {
  // ... campos existentes ...
  bytes AdminAddress       = 15 [json_name = "adminAddress"];
  bytes ControllerAddress  = 16 [json_name = "controllerAddress"];
}
```

`ControllerAddress` segue a mesma convenção dos demais endereços: bytes de comprimento fixo (validado por `pubkeyConv.Len()`); vazio significa "sem restrição".

## 6. Regra de runtime

Aplicada em `core/kapp/accounts/accounts.go::Transfer`, imediatamente após o check de `IsPaused`:

```
Se kda.ControllerAddress for vazio:
    comportamento atual (sem restrição).

Caso contrário:
    Permite se sender == ControllerAddress.
    Permite se cType == SmartContractType  E  tc.ToAddress == ControllerAddress.
    Caso contrário: rejeita com receipt e novo result code Transaction_TransferRestricted.
```

### 6.1 Tabela de cenários

| Caminho | Sender | Recipient | cType | Resultado |
|---|---|---|---|---|
| EOA → EOA direto | user A | user B | TransferContractType | **Bloqueia** |
| User → controller, valor anexado em SC call | user | controller | SmartContractType | Permite |
| User → SC qualquer, valor anexado | user | otherSC | SmartContractType | **Bloqueia** |
| Controller → user via `kdaTransfer` builtin | controller | user | TransferContractType | Permite |
| Controller → user via BlockChainHook | controller | user | TransferContractType | Permite |
| SC qualquer → user via `kdaTransfer` | otherSC | user | TransferContractType | **Bloqueia** |
| User → controller via TransferContract direto | user | controller | TransferContractType | **Bloqueia** (controller não reage em tempo real) |
| Controller → outro SC, valor anexado | controller | otherSC | SmartContractType | Permite |

### 6.2 Por que bloquear a deposição via TransferContract direto?

Quando o usuário envia um `TransferContract` para o controller sem invocar nenhum endpoint, o controller recebe os tokens em saldo, mas nenhum código do controller executa. A contabilidade interna do contrato fica fora de sincronia com o saldo on-chain — exatamente o tipo de inconsistência que o feature pretende impedir. Depósitos pelo usuário devem usar uma chamada com valor anexado.

## 7. Plano de implementação

1. **Proto:** adicionar `ControllerAddress` em `KDAData` e em `CreateAssetContract`; adicionar `SetController = 19` em `AssetTriggerContract.EnumTriggerType`.
2. **Regerar Go:** rodar `buildProto.sh`.
3. **Result code e erros:** adicionar `Transaction_TransferRestricted` em `transaction.proto`, `ErrFieldTransferRestricted` em `common/constants.go`, `ErrTransferRestrictedToController` e `ErrInvalidControllerAddr` em `core/process/errors`.
4. **CreateAsset:** copiar `ControllerAddress` do contrato para o `KDAData`; validar comprimento quando não-vazio em `assetHelper.ValidateCreateAsset`.
5. **AssetTrigger handler:** implementar `handleSetController` em `core/kapp/kda/trigger.go` — owner-only, exige `asset.ControllerAddress` vazio, valida o endereço informado.
6. **Runtime check:** implementar a regra em `accounts.Transfer` após o check de `IsPaused`.
7. **Testes:** cobrir cada linha da tabela de cenários em `accounts_test.go`, e o `SetController` (sucesso, segunda chamada bloqueada, sender não-owner, endereço inválido).

## 8. Fora de escopo

- **Allowances / approvals estilo ERC-20.** A regra é o portão; o controller faz toda a permissão em sua própria storage.
- **Rotação de controller.** Imutável; evolução via proxy/upgrade na camada SC.
- **Restrições para KLV.** KLV não é KDA; o campo não se aplica.
- **Migrações em massa de tokens existentes.** Tokens emitidos antes do feature continuam sem controller; não há retrofit automático.
