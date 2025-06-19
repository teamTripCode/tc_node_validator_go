"""
Gestor principal del cluster Go
Maneja el ciclo de vida de las instancias del cluster
Compatible con Windows, Linux y macOS mediante detección runtime
"""

import subprocess
import time
import os
import sys
from typing import List, Dict, Optional, Any
import logging
from config.settings import CLUSTER_CONFIG, get_os_config

# Importación condicional de signal para compatibilidad
import signal

logger = logging.getLogger(__name__)

class GoAppManager:
    """Gestor de aplicaciones Go en cluster - Compatible multi-plataforma"""
    
    def __init__(self):
        self.processes: List[Dict[str, Any]] = []
        self.ports = CLUSTER_CONFIG['ports']
        self.seed_node = CLUSTER_CONFIG['seed_node']
        self.os_config = get_os_config()
        
        # Detección de plataforma en runtime
        self.is_windows = sys.platform.startswith('win')
        self.is_posix = hasattr(os, 'fork')  # Unix-like systems
        
        # Configuración de señales según plataforma
        self.TERM_SIGNAL = signal.SIGTERM
        # SIGKILL solo existe en sistemas Unix, usar SIGTERM como fallback en Windows
        self.KILL_SIGNAL = getattr(signal, 'SIGKILL', signal.SIGTERM)
        
        # Log de plataforma detectada
        platform_info = self._get_platform_info()
        logger.info(f"Plataforma detectada: {platform_info}")
        
    def _get_platform_info(self) -> str:
        """Obtener información detallada de la plataforma"""
        if self.is_windows:
            try:
                version = sys.getwindowsversion()
                return f"Windows {version.major}.{version.minor}"
            except:
                return "Windows (versión desconocida)"
        else:
            return f"{sys.platform} (POSIX: {self.is_posix})"
    
    def create_data_directories(self) -> bool:
        """Crear directorios de datos necesarios"""
        try:
            # Crear directorio base
            base_dir = CLUSTER_CONFIG['data_base_dir']
            if not os.path.exists(base_dir):
                os.makedirs(base_dir)
                logger.info(f"Creado directorio base: {base_dir}")
            
            # Crear directorio principal
            principal_dir = CLUSTER_CONFIG['principal_dir']
            if not os.path.exists(principal_dir):
                os.makedirs(principal_dir)
                logger.info(f"Creado directorio principal: {principal_dir}")
            
            # Crear directorios para delegados
            for i in range(2, len(self.ports) + 1):
                delegate_dir = f"{CLUSTER_CONFIG['delegate_prefix']}{i:02d}"
                if not os.path.exists(delegate_dir):
                    os.makedirs(delegate_dir)
                    logger.debug(f"Creado directorio delegado: {delegate_dir}")
                    
            return True
            
        except Exception as e:
            logger.error(f"Error creando directorios: {e}")
            return False
    
    def _create_process(self, cmd: List[str]) -> subprocess.Popen:
        """Crear proceso de manera compatible con todas las plataformas"""
        base_kwargs = {
            'stdout': subprocess.PIPE,
            'stderr': subprocess.PIPE,
            'text': True,
            'bufsize': 1,
            'universal_newlines': True,
            'encoding': 'utf-8'
        }
        
        if self.is_windows:
            # Windows: Usar CREATE_NO_WINDOW para procesos en background
            base_kwargs['creationflags'] = subprocess.CREATE_NO_WINDOW
        else:
            # Unix/Linux: Usar setsid si está disponible para mejor control de grupo
            try:
                # Verificación runtime más robusta para setsid
                setsid_func = getattr(os, 'setsid', None)
                if setsid_func is not None and callable(setsid_func):
                    base_kwargs['preexec_fn'] = setsid_func
            except AttributeError:
                # setsid no disponible, continuar sin él
                pass
        
        return subprocess.Popen(cmd, **base_kwargs)
    
    def start_instance(self, port: int, datadir: str, is_principal: bool = False) -> bool:
        """Iniciar una instancia específica del cluster"""
        try:
            cmd = [
                "go", "run", "main.go", 
                f"-port={port}", 
                f"-datadir={datadir}", 
                "-verbose=true", 
                f"-seed={self.seed_node}"
            ]
            
            # Crear proceso usando método compatible
            process = self._create_process(cmd)
            
            # Registrar la instancia
            instance_info = {
                'process': process,
                'port': port,
                'type': 'PRINCIPAL' if is_principal else f'DELEGADO-{len(self.processes):02d}',
                'datadir': datadir,
                'started_at': time.time()
            }
            
            self.processes.append(instance_info)
            
            logger.info(f"Iniciada instancia {instance_info['type']} en puerto {port}")
            return True
            
        except Exception as e:
            logger.error(f"Error iniciando instancia en puerto {port}: {e}")
            return False
    
    def start_all_instances(self) -> bool:
        """Iniciar todas las instancias del cluster"""
        logger.info("Iniciando cluster completo...")
        
        if not self.create_data_directories():
            return False
        
        try:
            # Iniciar nodo principal
            if not self.start_instance(self.ports[0], CLUSTER_CONFIG['principal_dir'], True):
                logger.error("Falló el inicio del nodo principal")
                return False
            
            # Esperar que el principal esté listo
            logger.info(f"Esperando {CLUSTER_CONFIG['startup_delay']} segundos para que el principal esté listo...")
            time.sleep(CLUSTER_CONFIG['startup_delay'])
            
            # Iniciar delegados
            for i, port in enumerate(self.ports[1:], 2):
                datadir = f"{CLUSTER_CONFIG['delegate_prefix']}{i:02d}"
                if not self.start_instance(port, datadir, False):
                    logger.warning(f"Error iniciando delegado en puerto {port}")
                time.sleep(CLUSTER_CONFIG['instance_delay'])
            
            active_instances = len(self.processes)
            logger.info(f"Cluster iniciado con {active_instances} instancias")
            return active_instances > 0
            
        except Exception as e:
            logger.error(f"Error iniciando cluster: {e}")
            return False
    
    def _send_signal_to_process(self, process: subprocess.Popen, sig: int, force: bool = False) -> bool:
        """Enviar señal a proceso de manera compatible con todas las plataformas"""
        try:
            if self.is_windows:
                # En Windows, solo podemos usar terminate() o kill()
                if force or sig == self.KILL_SIGNAL:
                    process.kill()
                else:
                    process.terminate()
                
                # Esperar terminación en Windows
                timeout = 1 if force else 2
                try:
                    process.wait(timeout=timeout)
                    return True
                except subprocess.TimeoutExpired:
                    if not force:
                        logger.warning("Proceso no terminó en tiempo esperado")
                        return False
                    else:
                        logger.error("Proceso no pudo ser terminado forzadamente")
                        return False
                        
            else:
                # Sistemas Unix/Linux
                if self.is_posix:
                    # Verificar disponibilidad de killpg y getpgid de forma segura
                    try:
                        killpg_func = getattr(os, 'killpg', None)
                        getpgid_func = getattr(os, 'getpgid', None)
                        
                        if killpg_func and getpgid_func and callable(killpg_func) and callable(getpgid_func):
                            # Usar killpg si está disponible para terminar grupo de procesos
                            try:
                                getpgid_func(process.pid)  # Verificar que el proceso tenga grupo
                                killpg_func(getpgid_func(process.pid), sig)
                            except (ProcessLookupError, OSError) as e:
                                # Fallback a método directo si killpg falla
                                logger.debug(f"killpg falló, usando método directo: {e}")
                                if force:
                                    process.kill()
                                else:
                                    process.terminate()
                        else:
                            # Método directo si killpg no está disponible
                            if force:
                                process.kill()
                            else:
                                process.terminate()
                    except Exception as e:
                        logger.debug(f"Error con killpg, usando método directo: {e}")
                        if force:
                            process.kill()
                        else:
                            process.terminate()
                else:
                    # Método directo para sistemas no-POSIX
                    if force:
                        process.kill()
                    else:
                        process.terminate()
                
                return True
                
        except Exception as e:
            logger.error(f"Error enviando señal {sig} al proceso: {e}")
            return False
    
    def terminate_process(self, process: subprocess.Popen, force: bool = False) -> bool:
        """Terminar proceso de manera compatible (SIGTERM o SIGKILL)"""
        if process.poll() is not None:
            return True  # Proceso ya terminado
            
        signal_to_send = self.KILL_SIGNAL if force else self.TERM_SIGNAL
        return self._send_signal_to_process(process, signal_to_send, force)
    
    def stop_instance(self, instance: Dict[str, Any], force: bool = False) -> bool:
        """Detener una instancia específica"""
        try:
            process = instance['process']
            if process.poll() is None:
                action = "Forzando detención" if force else "Deteniendo"
                logger.info(f"{action} de instancia {instance['type']} en puerto {instance['port']}")
                
                return self.terminate_process(process, force)
            
            return False  # Proceso ya estaba terminado
            
        except Exception as e:
            logger.error(f"Error {'forzando' if force else 'deteniendo'} instancia {instance['type']}: {e}")
            return False
    
    def stop_all_instances(self, force_after_timeout: bool = True, timeout: float = 3.0) -> None:
        """Detener todas las instancias del cluster con opción de forzar"""
        logger.info("Deteniendo todas las instancias...")
        
        # Intentar detención normal primero
        for instance in self.processes:
            self.stop_instance(instance, force=False)
        
        if force_after_timeout:
            # Esperar el timeout
            logger.info(f"Esperando {timeout} segundos antes de forzar detención...")
            time.sleep(timeout)
            
            # Verificar cuáles siguen activos y forzar detención
            still_running = []
            for instance in self.processes:
                if instance['process'].poll() is None:
                    still_running.append(instance)
            
            if still_running:
                logger.warning(f"Forzando detención de {len(still_running)} instancias que no terminaron")
                for instance in still_running:
                    self.stop_instance(instance, force=True)
        
        # Limpiar lista de procesos
        self.processes.clear()
        logger.info("Todas las instancias han sido detenidas")
    
    def get_active_instances(self) -> List[Dict[str, Any]]:
        """Obtener lista de instancias activas"""
        active = []
        for instance in self.processes:
            if instance['process'].poll() is None:
                active.append(instance)
        return active
    
    def get_instance_status(self, instance: Dict[str, Any]) -> Dict[str, Any]:
        """Obtener estado detallado de una instancia"""
        process = instance['process']
        is_running = process.poll() is None
        
        status = {
            'is_running': is_running,
            'port': instance['port'],
            'type': instance['type'],
            'datadir': instance['datadir'],
            'pid': process.pid if is_running else None,
            'return_code': process.returncode if not is_running else None,
            'uptime': time.time() - instance['started_at'] if is_running else 0,
            'platform': 'Windows' if self.is_windows else 'Unix-like'
        }
        
        return status
    
    def get_cluster_status(self) -> Dict[str, Any]:
        """Obtener estado completo del cluster"""
        active_instances = self.get_active_instances()
        total_instances = len(self.processes)
        
        return {
            'total_instances': total_instances,
            'active_instances': len(active_instances),
            'platform': self._get_platform_info(),
            'instances': [self.get_instance_status(inst) for inst in self.processes]
        }
    
    def cleanup(self) -> None:
        """Limpieza final del gestor"""
        logger.info("Iniciando limpieza del gestor...")
        
        # Detener todas las instancias si hay alguna activa
        if self.processes:
            self.stop_all_instances(force_after_timeout=True, timeout=2.0)
        
        logger.info("Limpieza completada")