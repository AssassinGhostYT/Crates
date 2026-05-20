# Crates Plugin for Dragonfly

Un sistema de cajas (Crates) profesional y altamente visual para servidores de **Dragonfly** (Minecraft Bedrock).

## 🚀 Características

*   **Persistencia Total:** Las cajas, hologramas y recompensas se guardan en un archivo JSON y se restauran tras cada reinicio.
*   **Animación Trial Chamber:** Animación inspirada en la 1.21 de Minecraft. Los ítems giran y cambian sobre el cofre antes de revelar al ganador.
*   **Efectos Visuales Avanzados:**
    *   Partículas en forma de doble hélice para cajas comunes.
    *   **Mythic X:** Una "X" púrpura gigante que gira en 3D para cajas míticas.
    *   Explosiones y chispas de lava al abrir.
*   **Editor para Admins:** Cambia los premios simplemente metiendo ítems en el cofre y usando un comando.
*   **Anti-Exploit:** Los ítems de la animación están protegidos; nadie puede recogerlos hasta que el servidor los entregue.
*   **Sistema de Borrado Limpio:** Al eliminar una caja, el inventario se vacía automáticamente para no dejar basura.

## 🛠️ Comandos (Solo OPs)

*   `/crate set <rarity>`: Crea una caja en la posición que estás mirando.
*   `/crate save`: Guarda los ítems dentro del cofre como las nuevas recompensas.
*   `/crate remove`: Activa el modo borrado (rompe el cofre para eliminarlo).
*   `/crate give <player> <rarity> [amount]`: Entrega llaves a un jugador.
*   `/crate help`: Muestra el menú de ayuda.

## 💎 Rarezas Soportadas

*   **Common** (Gris)
*   **Rare** (Cian)
*   **Epic** (Rosa)
*   **Mythic** (Púrpura - Usa Cofre de Ender)

## 📦 Instalación

Este plugin está diseñado como un módulo de Go. Para integrarlo en tu servidor Dragonfly:

1. Importa el paquete en tu `main.go`.
2. Registra el sistema en la inicialización: `crates.Register(srv.World())`.
3. Asigna el handler a los jugadores que entren: `player.Handle(cratesHandler)`.

---
Desarrollado por **AssassinGhostYT**.
