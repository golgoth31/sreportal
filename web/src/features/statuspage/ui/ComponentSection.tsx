import type { PlatformComponent } from "../domain/types";
import { ComponentCard } from "./ComponentCard";

interface ComponentSectionProps {
  groupedComponents: Array<{
    group: string;
    components: PlatformComponent[];
  }>;
}

export function ComponentSection({
  groupedComponents,
}: ComponentSectionProps) {
  if (groupedComponents.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
        <p className="text-muted-foreground text-sm">
          No components configured for this portal.
        </p>
        <code className="text-xs bg-muted px-3 py-1.5 rounded">
          kubectl apply -f component.yaml
        </code>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {groupedComponents.map(({ group, components }) => (
        <div key={group}>
          <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
            {group}
          </h3>
          <div className="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
            {components.map((comp) => (
              <ComponentCard key={comp.name} component={comp} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
