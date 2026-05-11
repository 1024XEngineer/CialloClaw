import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";

export function BlogPage() {
  return (
    <Card className="rounded-[2.4rem] p-8 sm:p-10">
      <Badge lang="en">Blog</Badge>
      <h1 className="mt-5 text-4xl font-semibold tracking-[-0.03em]">博客 / 开发日志</h1>
      <p className="mt-4 max-w-2xl text-base leading-8 text-[color:var(--cc-ink-soft)]">
        这一页先作为 AIRI 风格框架中的博客入口壳子，后续可以接更新日志、里程碑、设计记录和开发周报。
      </p>
    </Card>
  );
}
