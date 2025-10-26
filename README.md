# tunnelled
A reverse-proxy to reverse-proxy a proxy :)

[!CAUTION]
This project is in early development and should not be used in production environments.
I do use it personally, and it works for my use case, but there are no guarantees.

# Why does this even exist?
This project is created because my ISP (Antel) changes your IP every 12 hours and I needed a "solution" for it.
I wanted my Minecraft players to be able to connect to my server and not get disconnected when my IP changes.
The idea is simple, you host "tunnelled-client" on a VM with a static IP. This VM connects to "tunnelled-server" running on your home server.
Then you connect to the "tunnelled-client" IP and it proxies the traffic to your home server.

This can probably be used for more than Minecraft as it's just a proxy for TCP traffic as it implements HAProxy protocol, but the main use case is for Minecraft servers.

# How does it work?
Simple, players connect to the tunnelled client hosted in the machine with the static IP.
Then, it proxies the traffic to your home server running the tunnelled server version.
Tunnelled server will now forward the traffic to your backend (for example, your Minecraft server).
The idea is that the connection dies between the tunnelled client and tunnelled server, but the connection between the player and the tunnelled client remains alive, and we queue the packets. And vice versa.
Once the tunnelled client reconnects to the tunnelled server, the traffic is forwarded again, and we sent the queued packets.

Basically:
- Player <-> Tunnelled Client <-> Tunnelled Server <-> Minecraft Server

# How to use it?
You need two machines:
1. A machine with a static IP (VPS, Cloud VM, etc) to run
2. Your home server where your Minecraft server is running

## On the static IP machine
1. Download the latest release from the [releases page](https://github.com/NekoCraftNW/tunnelled/releases).
2. Give execute permissions to the binary:
   ```bash
   chmod +x tunnelled
   ```
3. Run the binary with the following command: 
   ```bash
   ./tunnelled -type client
   ```

## On your home server
1. Download the latest release from the [releases page](https://github.com/NekoCraftNW/tunnelled/releases).
2. Give execute permissions to the binary:
   ```bash
   chmod +x tunnelled
   ```
3. Run the binary with the following command:
4. ```bash
   ./tunnelled -type server
   ```

# Proxy Protocol Support
We support HAProxy protocol v1 and v2.
You can enable it on the client (for proxying it with some solution like Papyrus or Cloudflare Spectrum), and on the server (for forwarding the real IP to your backend Minecraft server).

If you run the client with Spectrum behind it, enable HAProxy protocol v2 on the client.
If your target backend server (for example BungeeCord or Velocity) supports HAProxy protocol, enable it on the server too so players will have their real IP.