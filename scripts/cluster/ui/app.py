
"""
Aplicación principal del monitor de cluster
"""
from typing import Optional, Any
from threading import Thread

from kivy.app import App
from kivy.uix.boxlayout import BoxLayout
from kivy.uix.label import Label
from kivy.uix.button import Button
from kivy.uix.scrollview import ScrollView
from kivy.clock import Clock

from config.settings import APP_CONFIG, STATUS_MESSAGES
from core.manager import GoAppManager
from ui.widgets import InstanceWidget
from config.settings import COLORS, STATUS_ICONS

class GoClusterMonitorApp(App):
    def __init__(self, **kwargs: Any) -> None:
        super().__init__(**kwargs)
        self.manager: Optional[GoAppManager] = None
        self.monitor_thread: Optional[Thread] = None
        self.running: bool = True
        self.start_btn: Optional[Button] = None
        self.stop_btn: Optional[Button] = None
        self.status_label: Optional[Label] = None
        self.instances_layout: Optional[BoxLayout] = None
    
def build(self) -> BoxLayout:
    self.title = APP_CONFIG['title']
    
    # Layout principal
    main_layout = BoxLayout(orientation='vertical', padding=10, spacing=10)
    
    # Header
    header = BoxLayout(size_hint_y=None, height=80, spacing=10)
    
    title_label = Label(
        text=f"🎯 {self.title}",
        font_size=APP_CONFIG['font_size']['title'],
        bold=True,
        color=COLORS['text_primary']
    )
    
    # Botones de control
    control_layout = BoxLayout(size_hint_x=None, width=150, spacing=10)
    
    self.start_btn = Button(
        text="Iniciar Cluster",
        size_hint_y=None,
        height=35,
        background_color=COLORS['button_success']
    )
    
    self.stop_btn = Button(
        text="Detener Cluster",
        size_hint_y=None,
        height=35,
        background_color=COLORS['button_danger'],
        disabled=True
    )
    
    # Usar bind con supresión de warning de Pylance
    self.start_btn.bind(on_press=self.start_cluster)  # type: ignore
    self.stop_btn.bind(on_press=self.stop_cluster)    # type: ignore
    
    control_layout.add_widget(self.start_btn)
    control_layout.add_widget(self.stop_btn)
    
    header.add_widget(title_label)
    header.add_widget(control_layout)
    
    # Estado general
    self.status_label = Label(
        text=STATUS_MESSAGES['cluster_stopped'],
        size_hint_y=None,
        height=30,
        font_size=APP_CONFIG['font_size']['header'],
        color=COLORS['text_secondary']
    )
    
    # Lista de instancias
    self.instances_layout = BoxLayout(orientation='vertical', spacing=5)
    
    scroll = ScrollView()
    scroll.add_widget(self.instances_layout)
    
    main_layout.add_widget(header)
    main_layout.add_widget(self.status_label)
    main_layout.add_widget(scroll)
    
    # Configurar actualizaciones periódicas
    Clock.schedule_interval(self.update_interface, 60)  # Actualización completa
    Clock.schedule_interval(self.quick_update, 10)      # Actualización rápida
    
    return main_layout
    
    def start_cluster(self, instance: Button) -> None:
        """Iniciar el cluster"""
        if self.start_btn:
            self.start_btn.disabled = True
        if self.status_label:
            self.status_label.text = STATUS_MESSAGES['cluster_starting']
        
        self.manager = GoAppManager()
        self.monitor_thread = Thread(target=self.run_cluster)
        self.monitor_thread.daemon = True
        self.monitor_thread.start()
    
    def run_cluster(self) -> None:
        """Ejecutar el cluster en un hilo separado"""
        try:
            if self.manager is not None and self.manager.start_all_instances():
                Clock.schedule_once(lambda dt: self.on_cluster_started())
            else:
                Clock.schedule_once(lambda dt: self.on_cluster_error())
        except Exception as e:
            Clock.schedule_once(lambda dt: self.on_cluster_error(str(e)))
    
    def on_cluster_started(self) -> None:
        """Callback cuando el cluster inicia correctamente"""
        if self.status_label:
            self.status_label.text = STATUS_MESSAGES['cluster_active']
        if self.stop_btn:
            self.stop_btn.disabled = False
        self.update_instances_display()
    
    def on_cluster_error(self, error_msg: Optional[str] = None) -> None:
        """Callback cuando hay error al iniciar"""
        if self.status_label:
            self.status_label.text = f"{STATUS_ICONS['error']} Error: {error_msg or 'Error desconocido'}"
        if self.start_btn:
            self.start_btn.disabled = False
    
    def stop_cluster(self, instance: Button) -> None:
        """Detener el cluster"""
        if self.stop_btn:
            self.stop_btn.disabled = True
        if self.status_label:
            self.status_label.text = STATUS_MESSAGES['cluster_stopping']
        
        if self.manager:
            self.manager.stop_all_instances()
        
        if self.status_label:
            self.status_label.text = STATUS_MESSAGES['cluster_stopped']
        if self.start_btn:
            self.start_btn.disabled = False
        if self.instances_layout:
            self.instances_layout.clear_widgets()
    
    def update_instances_display(self) -> None:
        """Actualizar la visualización de instancias"""
        if not self.instances_layout:
            return
            
        self.instances_layout.clear_widgets()
        
        if self.manager and self.manager.processes:
            for instance in self.manager.processes:
                widget = InstanceWidget(instance)
                self.instances_layout.add_widget(widget)
    
    def update_interface(self, dt: float) -> bool:
        """Actualización completa del estado del cluster"""
        if self.manager and self.manager.processes and self.status_label:
            active_count = sum(1 for p in self.manager.processes if p['process'].poll() is None)
            
            if active_count == 0:
                self.status_label.text = STATUS_MESSAGES['cluster_stopped']
                if self.start_btn:
                    self.start_btn.disabled = False
                if self.stop_btn:
                    self.stop_btn.disabled = True
            elif active_count < len(self.manager.processes):
                self.status_label.text = f"{STATUS_ICONS['warning']} {active_count}/{len(self.manager.processes)} instancias activas"
            else:
                self.status_label.text = STATUS_MESSAGES['cluster_active']
        return True
    
    def quick_update(self, dt: float) -> bool:
        """Actualización rápida de los estados"""
        if hasattr(self, 'instances_layout') and self.instances_layout:
            for widget in self.instances_layout.children:
                if isinstance(widget, InstanceWidget):
                    widget.update_status()
        return True
    
    def on_stop(self) -> bool:
        """Cleanup al cerrar la aplicación"""
        self.running = False
        if self.manager:
            self.manager.stop_all_instances()
        return True
