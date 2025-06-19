import subprocess
import time
import os
import signal
import sys
import atexit
from threading import Thread

class GoAppManager:
    def __init__(self):
        self.processes = []
        self.ports =[3001, 3002, 3003, 3004, 3005, 3006, 3007, 3008, 3009, 
                     3010, 3011, 3012, 3013, 3014, 3015, 3016, 3017, 3018, 
                     3019, 3020, 3021, 3022, 3023]
        
    def create_data_directories(self):
        """Crear directorios de datos necesarios"""
        try:
            if not os.path.exists("data"):
                os.makedirs("data")
                print("✓ Directorio 'data' creado")
            
            for i in range(1, len(self.ports)):
                delegate_dir = f"data/delegate-{i:02d}"
                if not os.path.exists(delegate_dir):
                    os.makedirs(delegate_dir)
                    
        except Exception as e:
            print(f"❌ Error creando directorios: {e}")
            return False
        return True
    
    def start_instance(self, port, datadir, is_principal=False):
        """Iniciar una instancia específica en background"""
        try:
            cmd = ["go", "run", "main.go", f"-port={port}", f"-datadir={datadir}", "-verbose=true"]
            
            # Ejecutar en background sin ventana visible
            if os.name == 'nt':  # Windows
                process = subprocess.Popen(
                    cmd,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    creationflags=subprocess.CREATE_NO_WINDOW
                )
            else:  # Linux/Mac
                process = subprocess.Popen(
                    cmd,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    preexec_fn=os.setsid
                )
            
            self.processes.append({
                'process': process,
                'port': port,
                'type': 'PRINCIPAL' if is_principal else 'DELEGADO',
                'datadir': datadir
            })
            
            instance_type = "PRINCIPAL" if is_principal else f"DELEGADO {len(self.processes):02d}"
            print(f"🚀 {instance_type} iniciado en puerto {port}")
            return True
            
        except Exception as e:
            print(f"❌ Error iniciando instancia en puerto {port}: {e}")
            return False
    
    def start_all_instances(self):
        """Iniciar todas las instancias de forma paralela"""
        print("🔧 Preparando directorios...")
        if not self.create_data_directories():
            return False
        
        print(f"\n🚀 Iniciando {len(self.ports)} instancias...")
        print("=" * 50)
        
        # Lista para hilos de inicio
        threads = []
        
        # Función para iniciar instancia en hilo
        def start_worker(port, datadir, is_principal):
            self.start_instance(port, datadir, is_principal)
        
        # Iniciar instancia principal
        thread = Thread(target=start_worker, args=(3001, "data", True))
        threads.append(thread)
        thread.start()
        
        # Iniciar instancias delegadas
        for i, port in enumerate(self.ports[1:], 1):
            datadir = f"data/delegate-{i:02d}"
            thread = Thread(target=start_worker, args=(port, datadir, False))
            threads.append(thread)
            thread.start()
            time.sleep(0.1)  # Pequeño delay entre inicios
        
        # Esperar que todos los hilos terminen
        for thread in threads:
            thread.join()
        
        print("=" * 50)
        print(f"✅ {len(self.processes)} instancias iniciadas correctamente")
        print(f"📋 Puerto principal: 3001 (data)")
        print(f"📋 Puertos delegados: {', '.join(map(str, self.ports[1:]))}")
        
        return len(self.processes) == len(self.ports)
    
    def monitor_processes(self):
        """Monitorear procesos y mostrar estado"""
        active_count = 0
        for instance in self.processes:
            if instance['process'].poll() is None:
                active_count += 1
            else:
                print(f"⚠️  Instancia en puerto {instance['port']} se ha cerrado")
        
        return active_count
    
    def stop_all_instances(self):
        """Detener todas las instancias de forma rápida y limpia"""
        if not self.processes:
            return
            
        print("\n🛑 Cerrando todas las instancias...")
        
        # Terminar procesos de forma paralela
        for instance in self.processes:
            try:
                process = instance['process']
                if process.poll() is None:
                    if os.name == 'nt':  # Windows
                        process.terminate()
                    else:  # Linux/Mac
                        os.killpg(os.getpgid(process.pid), signal.SIGTERM)
            except Exception:
                pass
        
        # Esperar un poco y forzar cierre si es necesario
        time.sleep(1)
        
        for instance in self.processes:
            try:
                process = instance['process']
                if process.poll() is None:
                    if os.name == 'nt':
                        process.kill()
                    else:
                        os.killpg(os.getpgid(process.pid), signal.SIGKILL)
            except Exception:
                pass
        
        self.processes.clear()
        print("✅ Todas las instancias cerradas")
    
    def run(self):
        """Ejecutar el gestor principal"""
        try:
            if not self.start_all_instances():
                print("❌ Error al iniciar algunas instancias")
                return False
            
            print("\n💡 Presiona Ctrl+C para detener el cluster")
            print("📊 Monitoreando procesos...\n")
            
            # Loop principal de monitoreo
            while True:
                time.sleep(5)
                active = self.monitor_processes()
                if active == 0:
                    print("❌ Todas las instancias se han cerrado")
                    break
                    
        except KeyboardInterrupt:
            print("\n🔄 Cerrando cluster...")
        except Exception as e:
            print(f"❌ Error inesperado: {e}")
        finally:
            self.stop_all_instances()
            
        return True

# Variables globales para cleanup
manager = None

def cleanup():
    """Función de limpieza al salir"""
    global manager
    if manager:
        manager.stop_all_instances()

def signal_handler(sig, frame):
    """Manejador optimizado de señales"""
    print("\n🔄 Señal de interrupción recibida...")
    cleanup()
    sys.exit(0)

def main():
    global manager
    
    # Configurar limpieza automática
    atexit.register(cleanup)
    signal.signal(signal.SIGINT, signal_handler)
    if os.name != 'nt':
        signal.signal(signal.SIGTERM, signal_handler)
    
    print("🎯 Gestor de Cluster Go - Modo Background")
    print("=" * 50)
    
    manager = GoAppManager()
    
    try:
        success = manager.run()
        return 0 if success else 1
        
    except Exception as e:
        print(f"❌ Error crítico: {e}")
        return 1

if __name__ == "__main__":
    sys.exit(main())