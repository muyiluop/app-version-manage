import { message } from "antd";

export const showMessage = {
  success: (content: string) => {
    message?.success(content);
  },
  error: (content: string) => {
    message?.error(content);
  },
  warning: (content: string) => {
    message?.warning(content);
  },
  info: (content: string) => {
    message?.info(content);
  },
};
