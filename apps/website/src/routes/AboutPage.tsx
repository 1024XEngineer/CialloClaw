import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";

export function AboutPage() {
  return (
    <Card className="rounded-[2.4rem] p-8 sm:p-10">
      <Badge lang="en">About</Badge>
      <h1 className="mt-5 text-4xl font-semibold tracking-[-0.03em]">关于 CialloClaw</h1>
      <p className="mt-4 max-w-2xl text-base leading-8 text-[color:var(--cc-ink-soft)]">
        这里会承接项目说明、隐私政策、使用条款和团队信息。当前先把导航框架和页面位置铺好。
      </p>
    </Card>
  );
}
