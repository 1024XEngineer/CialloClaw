import { MascotHero } from "@/components/MascotHero";
import { WebsiteLayout } from "@/components/WebsiteLayout";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";

export function CharacterPage() {
  return (
    <WebsiteLayout>
      <section className="grid gap-8 lg:grid-cols-[1fr_0.95fr] lg:items-center">
        <Card className="rounded-[2.4rem] p-8 sm:p-10">
          <Badge>Character</Badge>
          <h1 className="mt-5 text-4xl font-semibold tracking-[-0.03em]">CialloClaw Mascot</h1>
          <p className="mt-4 max-w-2xl text-base leading-8 text-[color:var(--cc-ink-soft)]">
            这一页对应 AIRI 文档站中的角色入口。后续可以补悬浮球设定、形象海报、表情状态和品牌视觉规范。
          </p>
        </Card>
        <Card className="rounded-[2.4rem] p-4 sm:p-8">
          <MascotHero />
        </Card>
      </section>
    </WebsiteLayout>
  );
}
