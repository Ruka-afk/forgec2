"use client";

import { useI18n } from "@/lib/i18n";
import { useUrlState } from "@/lib/hooks/useUrlState";
import { PageContainer } from "@/components/ui/page-container";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Activity, ListTodo, Bell } from "lucide-react";
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
        <TabsList>
          <TabsTrigger value="stream" className="gap-1.5"><Activity className="size-3.5" />{t("events.tab_stream")}</TabsTrigger>
          <TabsTrigger value="tasks" className="gap-1.5"><ListTodo className="size-3.5" />{t("events.tab_tasks")}</TabsTrigger>
          <TabsTrigger value="alerts" className="gap-1.5"><Bell className="size-3.5" />{t("events.tab_alerts")}</TabsTrigger>
        </TabsList>
        <TabsContent value="stream" className="mt-4">
          <EventsStream />
        </TabsContent>
        <TabsContent value="tasks" className="mt-4">
          <TasksPageContent embedded />
        </TabsContent>
        <TabsContent value="alerts" className="mt-4">
          <NotificationsPageContent embedded />
        </TabsContent>
      </Tabs>
    </PageContainer>
  );
}
