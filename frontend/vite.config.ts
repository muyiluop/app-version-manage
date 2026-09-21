import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    // antd 本身体积较大，拆分后仍会超过默认 500kB 阈值，这里放宽到 1.2MB
    chunkSizeWarningLimit: 1200,
    // 把体积较大的第三方库单独拆包，避免业务改动导致整包缓存失效。
    rollupOptions: {
      output: {
        manualChunks: {
          react: ["react", "react-dom", "react-router-dom"],
          antd: ["antd", "@ant-design/icons"],
          query: ["@tanstack/react-query"],
        },
      },
    },
  },
  server: {
    proxy: {
      // 后端 API（含 /api/static/logos 图标）统一走这里
      "/api": {
        target: "http://localhost:9080/api",
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
});
