importScripts('wasm_exec.js');

const siap = new Promise((resolve, reject) => {
  self.addEventListener('gembok-siap', () => resolve(), { once: true });
  const go = new Go();
  const muat = WebAssembly.instantiateStreaming
    ? WebAssembly.instantiateStreaming(fetch('gembok.wasm'), go.importObject).catch(() =>
        fetch('gembok.wasm').then(r => r.arrayBuffer()).then(b => WebAssembly.instantiate(b, go.importObject)))
    : fetch('gembok.wasm').then(r => r.arrayBuffer()).then(b => WebAssembly.instantiate(b, go.importObject));
  muat.then(({ instance }) => go.run(instance)).catch(reject);
});

self.onmessage = async (e) => {
  const { id, fn, args } = e.data;
  try {
    await siap;
    const hasil = await self.gembok[fn](...args);
    self.postMessage({ id, ok: true, hasil });
  } catch (err) {
    self.postMessage({ id, ok: false, galat: String(err && err.message || err) });
  }
};
