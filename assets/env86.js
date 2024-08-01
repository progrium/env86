
import * as duplex from "./duplex.min.js";

export async function boot(imageURL, options) {
    const resp = await fetch(`${imageURL}/image.json`);
    const imageConfig = await resp.json();
    
    let config = Object.assign(imageConfig, options);
    const initStateChunks = imageConfig["initial_state_parts"];
    if (initStateChunks) {
        const stateChunkURLs = generateRange(initStateChunks).map(suffix => `${imageURL}/state/initial.state.${suffix}`);
        config["initial_state"] = {
            buffer: await downloadChunks(stateChunkURLs)
        };
    }

    if (!config["wasm_path"]) {
        const url = new URL(import.meta.url);
        const path = url.pathname.split("/");
        path.pop();
        path.push("v86.wasm");
        url.pathname = path.join("/");
        config["wasm_path"] = url.toString();
    }
    config["autostart"] = true;
    if (!config["memory_size"]) {
        config["memory_size"] = 512 * 1024 * 1024; // 512MB
    }
    if (!config["vga_memory_size"]) {
        config["vga_memory_size"] = 8 * 1024 * 1024; // 8MB
    }
    if (!config["filesystem"]) {
        config["filesystem"] = {
            baseurl: `${imageURL}/fs/`,
            basefs: `${imageURL}/fs.json`,
        };
    }

    ["bios", "vga_bios", "initrd", "bzimage"].forEach(key => {
        if (config[key] && config[key]["url"].startsWith("./")) {
            config[key]["url"] = imageURL+config[key]["url"].slice(1)
        }
    });
    
    let peer = undefined;
    if (config["control_url"]) {
        peer = await duplex.connect(config["control_url"], new duplex.CBORCodec());
        const resp = await peer.call("config");
        config = Object.assign(config, resp.value);
    }

    const vm = new V86(config);

    if (peer) {
        let tty = undefined;
        const enc = new TextEncoder();
        vm.add_listener("emulator-loaded", async () => {
            peer.call("loaded", []);
        
            if (config.EnableTTY && tty === undefined) {
                const tty = await peer.call("tty");
                vm.add_listener("serial0-output-byte", (b) => {
                    tty.channel.write(enc.encode(String.fromCharCode(b)));
                });
                (async () => {
                    const buf = new Uint8Array(1024);
                    while (true) {
                        const n = await tty.channel.read(buf);
                        if (n === null) {
                            break;
                        }
                        const data = new Uint8Array(buf.slice(0, n));
                        vm.serial_send_bytes(0, data);
                    }
                })();
            }
        });

        peer.handle("pause", duplex.handlerFrom(() => vm.stop()));
        peer.handle("unpause", duplex.handlerFrom(() => vm.run()));
        peer.handle("save", duplex.handlerFrom(async () => {
            const buf = await vm.save_state();
            return new Uint8Array(buf);
        }));
        peer.handle("restore", duplex.handlerFrom((data) => {
            vm.restore_state(data.buffer);
        }));
        peer.handle("sendText", duplex.handlerFrom((text) => {
            vm.keyboard_send_text(text);
        }));
        peer.handle("setScale", duplex.handlerFrom((x, y) => {
            vm.screen_set_scale(x, y);
        }));
        peer.handle("setFullscreen", duplex.handlerFrom(() => {
            vm.screen_go_fullscreen();
        }));
        peer.handle("mac", duplex.handlerFrom(() => {
            return vm.v86.cpu.devices.net.mac.map(el => el.toString(16).padStart(2, "0")).join(":");
        }));
        peer.handle("screenshot", duplex.handlerFrom(() => {
            const image = vm.screen_make_screenshot();
            if (image === null) {
                return null;
            }
            let binary = atob(image.src.split(',')[1]);
            var array = [];
            for (var i = 0; i < binary.length; i++) {
                array.push(binary.charCodeAt(i));
            }
            return new Uint8Array(array);
        }));

        

        peer.respond();
    }
    
    vm.config = config;
    return vm;
}


async function downloadChunks(urls) {
    const responses = await Promise.all(urls.map(url => fetch(url).then(response => {
        if (response.status === 404) {
            return null;
        }
        return response;
    })));
    const validResponses = responses.filter(response => response !== null);
    const arrayBuffers = await Promise.all(validResponses.map(response => response.arrayBuffer()));

    const totalLength = arrayBuffers.reduce((sum, buffer) => sum + buffer.byteLength, 0);
    const concatenatedBuffer = new Uint8Array(totalLength);
    let offset = 0;
    for (const buffer of arrayBuffers) {
        concatenatedBuffer.set(new Uint8Array(buffer), offset);
        offset += buffer.byteLength;
    }
    return concatenatedBuffer.buffer;
}

function generateRange(x) {
    if (typeof x !== 'number' || x < 0) {
        throw new Error('Input must be a non-negative number');
    }

    const result = [];
    for (let i = 0; i < x; i++) {
        result.push(i.toString());
    }

    return result;
}