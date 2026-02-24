import { generateFiles } from "fumadocs-openapi";

await generateFiles({
  input: ["../../docs/openapi.json"],
  output: "./content/docs/api",
  per: 'operation',   // one MDX file per RPC method
  includeDescription: true,
  // Don't use groupBy: 'tag' — use name instead
  name: (output, document) => {
    if (output.type === 'operation') {
      const op = document.paths?.[output.item.path]?.[output.item.method];
      
      // Always use tags[0] — that's YOUR tag, not the service name
      const folder = (op?.tags?.[0] ?? 'misc')
        .toLowerCase()
        .replace(/\s+/g, '-');
      
      // Use operationId's method part as filename
      // e.g. "fractalengine.rpc.v1.FractalEngineRpcService.DogeConfirm" → "doge-confirm"
      const method = output.item.operationId?.split('.').pop() ?? output.item.path.split('/').pop();
      const filename = method
        .replace(/([A-Z])/g, (m, l, i) => (i > 0 ? '-' : '') + l.toLowerCase())
        .replace(/^-/, '');
      
      return `${folder}/${filename}`;
    }
    return output.item.path;
  },
});