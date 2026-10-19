export const GENERATE_TABS = ["payload", "packer", "stager", "builds", "profiles"] as const;
export type GenerateTab = (typeof GENERATE_TABS)[number];
