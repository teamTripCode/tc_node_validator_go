"""
Widget que representa una instancia del cluster con su estado y controles
"""

from kivy.uix.boxlayout import BoxLayout
from kivy.uix.label import Label
from kivy.uix.button import Button
from kivy.uix.widget import Widget
from kivy.graphics import Color, Rectangle
from time import time
import psutil

from config.settings import COLORS
from core.utils import format_uptime

class InstanceWidget(BoxLayout):
    def __init__(self, instance_info, **kwargs):
        super().__init__(**kwargs)
        self.instance_info = instance_info
        self.orientation = 'horizontal'
        self.size_hint_y = None
        self.height = 60
        self.padding = 5
        self.spacing = 10

        # Fondo del widget
        from kivy.clock import Clock
        def draw_bg(*args):
            if self.canvas is not None:
                with self.canvas.before:
                    Color(*COLORS['widget_background'])
                    self.rect = Rectangle(pos=self.pos, size=self.size)
        Clock.schedule_once(draw_bg)
        self.bind(pos=self.update_rect, size=self.update_rect)
        
        # Layout de información
        info_layout = BoxLayout(orientation='vertical', size_hint_x=0.7)
        
        self.title_label = Label(
            text=f"{instance_info['type']} - Puerto {instance_info['port']}",
            font_size='14sp',
            bold=True,
            color=COLORS['text_primary'],
            halign='left'
        )
        
        self.status_label = Label(
            text="Verificando estado...",
            font_size='12sp',
            color=COLORS['text_secondary'],
            halign='left'
        )
        
        info_layout.add_widget(self.title_label)
        info_layout.add_widget(self.status_label)
        
        # Indicador de estado
        self.status_indicator = Widget(size_hint_x=None, width=20)
        # Ensure the canvas exists before drawing
        def draw_initial_circle(*args):
            if self.status_indicator.canvas is not None:
                with self.status_indicator.canvas:
                    Color(0.5, 0.5, 0.5, 1)
                    self.status_circle = Rectangle(pos=(0, 0), size=(15, 15))
        self.status_indicator.bind(parent=lambda *a: draw_initial_circle())
        if self.status_indicator.parent is not None:
            draw_initial_circle()
        
        # Botón de logs
        self.logs_btn = Button(
            text="Ver Logs",
            size_hint_x=None,
            width=100,
            background_color=COLORS['info']
        )
        self.logs_btn.bind(on_press=self.show_logs)
        
        self.add_widget(info_layout)
        self.add_widget(self.status_indicator)
        self.add_widget(self.logs_btn)
        
        self.update_status()
    
    def update_rect(self, *args):
        self.rect.pos = self.pos
        self.rect.size = self.size
    
    def update_status(self):
        """Actualizar el estado visual de la instancia"""
        try:
            process = self.instance_info['process']
            is_running = process and process.poll() is None
            
            if is_running:
                port_active = self.is_port_in_use(self.instance_info['port'])
                if port_active:
                    status = "ACTIVO"
                    color = COLORS['success']
                    uptime = time.time() - self.instance_info['started_at']
                    status_text = f"Estado: {status} | Uptime: {format_uptime(uptime)}"
                else:
                    status = "INICIANDO"
                    color = COLORS['warning']
                    status_text = "Estado: Iniciando..."
                
                self.logs_btn.disabled = False
            else:
                status = "INACTIVO"
                color = COLORS['error']
                status_text = "Estado: Inactivo"
                self.logs_btn.disabled = True
            
            self.status_label.text = status_text
            self.update_indicator_color(color)
            
        except Exception as e:
            self.status_label.text = f"Error: {str(e)}"
            self.update_indicator_color(COLORS['error'])
    
    def update_indicator_color(self, color):
        """Actualizar el color del indicador visual"""
        if self.status_indicator.canvas is not None:
            self.status_indicator.canvas.clear()
            with self.status_indicator.canvas:
                Color(*color)
                self.status_circle = Rectangle(
                    pos=(self.status_indicator.x + 2, self.status_indicator.center_y - 7),
                    size=(15, 15)
                )
    
    def is_port_in_use(self, port):
        """Verificar si el puerto está en uso"""
        try:
            for conn in psutil.net_connections():
                if hasattr(conn, 'laddr') and conn.laddr and conn.laddr.port == port:
                    if conn.status == 'LISTEN':
                        return True
            return False
        except Exception:
            return False
    
    def show_logs(self, *args):
        """Mostrar los logs de la instancia"""
        from ui.widgets.log_popup import LogViewerPopup
        popup = LogViewerPopup(self.instance_info)
        popup.open()