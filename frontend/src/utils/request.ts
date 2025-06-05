import axios from "axios";
import { showMessage } from "./message";

// 设置基础URL
axios.defaults.baseURL = import.meta.env.VITE_API_BASE_URL;

// 添加请求拦截器
axios.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem("token");
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 添加响应拦截器
axios.interceptors.response.use(
  (response) => {
    return response;
  },
  (error) => {
    if (error.response) {
      switch (error.response.status) {
        case 401:
          showMessage.error("未登录或登录已过期，请重新登录");
          localStorage.removeItem("token");
          window.location.href = "/login";
          break;
        case 403:
          showMessage.error("没有权限执行此操作");
          break;
        case 404:
          showMessage.error(error.response.data?.error || "请求的资源不存在");
          break;
        case 500:
          showMessage.error(error.response.data?.error || "服务器内部错误");
          break;
        default:
          showMessage.error(error.response.data?.error || "操作失败");
      }
    } else if (error.request) {
      showMessage.error("网络请求失败，请检查网络连接");
    } else {
      showMessage.error("请求配置错误");
    }
    return Promise.reject(error);
  }
);
