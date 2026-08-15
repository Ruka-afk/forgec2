"use client";

import { useI18n } from "@/lib/i18n";
import { useUrlState } from "@/lib/hooks/useUrlState";
import { PageContainer } from "@/components/ui/page-container";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { EVENTS_TABS, type EventsTab } from "./types";
import EventsStream from "./EventsStream";
import TasksPageContent from "../../tasks/TasksPageContent";
import NotificationsPageContent from "../../notifications/NotificationsPageContent";

export default function EventsPageContent() {
  const { t } = useI18n();
  const [tab, setTab] = useUrlState<EventsTab>("tab", "stream", EVENTS_TABS);

  return (
    <PageContainer title={t("events.title")} subtitle={t("events.subtitle")}>
      <Tabs value={tab} onValueChange={(v) => { if (v && (EVENTS_TABS as readonly string[]).includes(v)) setTab(v as EventsTab); }}>
        <TabsList className="mb-4">
          <TabsTrigger value="stream">{t("events.tab_stream")}</TabsTrigger>
          <TabsTrigger value="tasks">{t("events.tab_tasks")}</TabsTrigger>
          <TabsTrigger value="alerts">{t("events.tab_alerts")}</TabsTrigger>
        </TabsList>
        <TabsContent value="stream" className="mt-0">
          <EventsStream />
        </TabsContent>
        <TabsContent value="tasks" className="mt-0">
          <TasksPageContent embedded />
        </TabsContent>
        <TabsContent value="alerts" className="mt-0">
          <NotificationsPageContent embedded />
        </TabsContent>
      </Tabs>
    </PageContainer>
  );
}
