import { Button, Result } from "antd";
import { useNavigate } from "react-router-dom";

/** 兜底 404。 */
export default function NotFound() {
  const navigate = useNavigate();
  return (
    <Result
      status="404"
      title="404"
      subTitle="页面不存在或已被移除"
      extra={
        <Button type="primary" onClick={() => navigate("/apps", { replace: true })}>
          返回应用列表
        </Button>
      }
    />
  );
}
